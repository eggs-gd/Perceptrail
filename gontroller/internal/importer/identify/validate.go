package identify

import (
	"fmt"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// ValidatorStore: what validate asks of the model — which item a group is (the
// model's identity rules), and a group that is no item remembered as ignored
type ValidatorStore interface {
	ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error)
	ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error)
	Ignore(files []*dto.FileDto) error
}

// validate: the validate step's logic — the group's identity in the DB
// (Walker.puml): not media or broken, the model ignores it; else the model says
// which item it is (links, same / changed / moved / duplicate). One at a time: two
// groups never decide one identity at once.
type validate struct {
	db     ValidatorStore
	logger *l.Logger
}

func (v *validate) Decorate(g *draft) (*draft, error) {
	if g.Exif[0] == nil {
		return nil, fmt.Errorf("no metadata for the main file %s", g.Files[0].Path)
	}
	// Decided now (the model saves the rows): a change is no longer pending
	for _, f := range g.Files {
		f.Changed = false
	}
	main := g.Files[0]
	if !g.isMedia() { // nothing to show: remembered, so the gate skips it from now on
		if err := v.db.Ignore(g.Files); err != nil {
			return nil, err
		}
		return nil, chain.ErrSkippedItem
	}
	if g.Key != "" {
		item, err := v.db.ValidateAsset(g.Key, g.Files, g.Hash)
		if err != nil {
			return nil, err
		}
		item.MetaHash, item.Kind = g.MetaHash, dto.AssetKind(g.Kind, g.Files) // saved by the closer
		g.Item = item
		return g, nil
	}
	if reason := broken(g); reason != "" {
		v.logger.Warn("Broken file: ignored until it changes", l.String("file", main.Path), l.String("reason", reason))
		if err := v.db.Ignore(g.Files); err != nil {
			return nil, err
		}
		return nil, chain.ErrSkippedItem
	}

	// Moved items go on too: the cheap stage is cheap, and their preview path
	// changed with them. Skipping outputs that already exist is the expensive
	// stage's business.
	item, err := v.db.ValidateGroup(g.Files, g.Hash)
	if err != nil {
		return nil, err
	}
	item.Kind = dto.AssetKind("", g.Files) // saved by the closer
	g.Item = item
	return g, nil
}

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
