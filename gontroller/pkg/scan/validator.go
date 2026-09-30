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
	item.MetaHash = g.MetaHash // saved by the closer
	g.Item = item
	return g, nil
}

func (v *validator) Stop() {}
