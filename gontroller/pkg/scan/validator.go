package scan

import (
	"fmt"

	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// validator: the group's identity in the DB (Walker.puml). Links the files to the
// main file, then same / changed / moved / duplicate -> the item.
type validator struct {
	logger *l.Logger
}

func NewValidator(chin <-chan *flow.RawItem, chout chan<- *flow.RawItem, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, newValidator(logger))
}

func newValidator(logger *l.Logger) *validator {
	return &validator{logger: logger}
}

// Decorate sets the item of the group (ranked by mime: the main file first)
func (v *validator) Decorate(g *flow.RawItem) (*flow.RawItem, error) {
	main := g.Files[0]

	if !g.IsMedia() { // nothing to show: remembered, so the gate skips it from now on
		for _, f := range g.Files {
			f.SetIgnored()
		}
		if _, err := filesProxy.UpdateFiles(g.Files); err != nil {
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
		if old, err := itemsProxy.GetItemByGuid(f.GUID); err == nil {
			if err := itemsProxy.DeleteItem(old); err != nil {
				return nil, err
			}
			v.logger.Info("Former main file is a sidecar now", l.String("file", f.Path), l.String("main", main.Path))
		}
	}
	if _, err := filesProxy.UpdateFiles(g.Files); err != nil {
		return nil, err
	}

	// Moved items go on too: the cheap stage is cheap, and their preview path
	// changed with them. Skipping outputs that already exist is the expensive
	// stage's business.
	item, _, err := itemsProxy.ValidateFile(main, g.Exif[0])
	if err != nil {
		return nil, err
	}

	g.Item = item
	return g, nil
}

// keyed: the source knows the identity (an Apple Photos asset UUID): the item's
// GUID is the key, every file links to it, whatever the main file is
func (v *validator) keyed(g *flow.RawItem) (*flow.RawItem, error) {
	for _, f := range g.Files {
		// An item of the file's own from before (the generic grouper read the
		// library's originals): the asset's item replaces it
		if f.GUID != g.Key {
			if old, err := itemsProxy.GetItemByGuid(f.GUID); err == nil {
				if err := itemsProxy.DeleteItem(old); err != nil {
					return nil, err
				}
			}
		}
		f.LinkToItem(g.Key)
	}
	if _, err := filesProxy.UpdateFiles(g.Files); err != nil {
		return nil, err
	}
	item, err := itemsProxy.ValidateKeyed(g.Key, g.Files[0], g.Exif[0])
	if err != nil {
		return nil, err
	}
	item.MetaHash, item.Kind = g.MetaHash, g.Kind // saved by the closer
	g.Item = item
	return g, nil
}

func (v *validator) Stop() {}

// broken: why the main file of a group cannot be a photo, "" if it can. exiftool
// read it and says so (Error: "File format error", "File is empty"), or it is an
// image with no size at all (a JPEG cut after its header). A keyed group (Apple
// Photos) is not judged by its file: the library's DB is the truth, and its
// derivatives may be fine.
func broken(g *flow.RawItem) string {
	if e := g.Exif[0]["Error"]; len(e) > 0 {
		return string(e)
	}
	if g.Kinds[0] == flow.KindImage || g.Kinds[0] == flow.KindRaw {
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
func (v *validator) ignoreBroken(g *flow.RawItem, reason string) error {
	main := g.Files[0]
	v.logger.Warn("Broken file: ignored until it changes", l.String("file", main.Path), l.String("reason", reason))
	if item, err := itemsProxy.GetItemByGuid(main.GUID); err == nil {
		if err := itemsProxy.DeleteItem(item); err != nil {
			return err
		}
	}
	for _, f := range g.Files {
		f.SetIgnored()
	}
	if _, err := filesProxy.UpdateFiles(g.Files); err != nil {
		return err
	}
	return chain.ErrSkippedItem
}
