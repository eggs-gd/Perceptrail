package identify

import (
	"fmt"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// ValidatorStore: what validate reads and writes — the item of a group (by its
// main file's hash or its key), the files' links, a superseded item deleted
type ValidatorStore interface {
	ValidateFile(main *dto.FileDto, hash string) (*dto.ItemDto, model.Outcome, error)
	ValidateKeyed(key string, main *dto.FileDto, hash string) (*dto.ItemDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	GetItemByGuid(guid string) (*dto.ItemDto, error)
	DeleteItem(item *dto.ItemDto) error
}

// validator: the group's identity in the DB (Walker.puml). Links the files to the
// main file, then same / changed / moved / duplicate -> the item.
type Validator struct {
	db     ValidatorStore
	logger *l.Logger
}

func NewValidator(db ValidatorStore, logger *l.Logger) *Validator {
	return &Validator{db: db, logger: logger}
}

// Decorate sets the item of the group (ranked by mime: the main file first)
func (v *Validator) Decorate(g *draft) (*draft, error) {
	main := g.Files[0]

	if !g.isMedia() { // nothing to show: remembered, so the gate skips it from now on
		for _, f := range g.Files {
			f.SetIgnored()
		}
		if _, err := v.db.UpdateFiles(g.Files); err != nil {
			return nil, err
		}
		return nil, chain.ErrSkippedItem
	}
	if g.Exif[0] == nil {
		return nil, fmt.Errorf("no metadata for the main file %s", main.Path)
	}

	if g.Key != "" {
		return v.keyed(g)
	}
	if reason := broken(g); reason != "" {
		return nil, v.ignoreBroken(g, reason)
	}

	for _, f := range g.Files {
		f.LinkTo(main)
	}
	// A file that was the main file of its own item is a sidecar now (e.g. a JPEG
	// imported alone, then its RAW appeared): that item goes
	for _, f := range g.Files[1:] {
		if old, err := v.db.GetItemByGuid(f.GUID); err == nil {
			if err := v.db.DeleteItem(old); err != nil {
				return nil, err
			}
			v.logger.Info("Former main file is a sidecar now", l.String("file", f.Path), l.String("main", main.Path))
		}
	}
	if _, err := v.db.UpdateFiles(g.Files); err != nil {
		return nil, err
	}

	// Moved items go on too: the cheap stage is cheap, and their preview path
	// changed with them. Skipping outputs that already exist is the expensive
	// stage's business.
	item, _, err := v.db.ValidateFile(main, g.Hash)
	if err != nil {
		return nil, err
	}

	g.Item = item
	return g, nil
}

// keyed: the source knows the identity (an Apple Photos asset UUID): the item's
// GUID is the key, every file links to it, whatever the main file is
func (v *Validator) keyed(g *draft) (*draft, error) {
	for _, f := range g.Files {
		// An item of the file's own from before (the plain folder's grouper read the
		// library's originals): the asset's item replaces it
		if f.GUID != g.Key {
			if old, err := v.db.GetItemByGuid(f.GUID); err == nil {
				if err := v.db.DeleteItem(old); err != nil {
					return nil, err
				}
			}
		}
		f.LinkToItem(g.Key)
	}
	if _, err := v.db.UpdateFiles(g.Files); err != nil {
		return nil, err
	}
	item, err := v.db.ValidateKeyed(g.Key, g.Files[0], g.Hash)
	if err != nil {
		return nil, err
	}
	item.MetaHash, item.Kind = g.MetaHash, g.Kind // saved by the closer
	g.Item = item
	return g, nil
}

func (v *Validator) Stop() {}

// broken: why the main file of a group cannot be a photo, "" if it can. exiftool
// read it and says so (Error: "File format error", "File is empty"), or it is an
// image with no size at all (a JPEG cut after its header). A keyed group (Apple
// Photos) is not judged by its file: the library's DB is the truth, and its
// derivatives may be fine.
func broken(g *draft) string {
	if e := g.Exif[0]["Error"]; len(e) > 0 {
		return string(e)
	}
	if g.Kinds[0] == kindImage || g.Kinds[0] == kindRaw {
		for _, tag := range []string{"ImageWidth", "ImageSize", "ExifImageWidth"} {
			if len(g.Exif[0][tag]) > 0 {
				return ""
			}
		}
		return "no image size"
	}
	return ""
}

// ignoreBroken: the group's files are remembered as ignored — the gate skips them
// until a file changes (then they are processed again); an item the file used to be
// (it got corrupted) goes
func (v *Validator) ignoreBroken(g *draft, reason string) error {
	main := g.Files[0]
	v.logger.Warn("Broken file: ignored until it changes", l.String("file", main.Path), l.String("reason", reason))
	if item, err := v.db.GetItemByGuid(main.GUID); err == nil {
		if err := v.db.DeleteItem(item); err != nil {
			return err
		}
	}
	for _, f := range g.Files {
		f.SetIgnored()
	}
	if _, err := v.db.UpdateFiles(g.Files); err != nil {
		return err
	}
	return chain.ErrSkippedItem
}
