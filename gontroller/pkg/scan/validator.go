package scan

import (
	"fmt"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// validator: the group's identity in the DB (Walker.puml). Links the files to the
// main file, then same / changed / moved / duplicate -> the item.
type validator struct {
	logger *l.Logger
	// outputsComplete: everything the transcoder makes for the item exists
	outputsComplete func(guid string) bool
}

// No transcoder yet: there is nothing to regenerate
func noOutputs(string) bool { return true }

func NewValidator(chin <-chan exifGroup, chout chan<- *RawItem, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, newValidator(logger))
}

func newValidator(logger *l.Logger) *validator {
	return &validator{logger: logger, outputsComplete: noOutputs}
}

func (v *validator) Decorate(g exifGroup) (*RawItem, error) {
	main := g.files[0]

	if !g.media { // nothing to show: remembered, so the gate skips it from now on
		for _, f := range g.files {
			f.SetIgnored()
		}
		if _, err := filesProxy.UpdateFiles(g.files); err != nil {
			return nil, err
		}
		return nil, chain.ErrSkippedItem
	}
	if g.exifs[0] == nil {
		return nil, fmt.Errorf("no metadata for the main file %s", main.Path)
	}

	for _, f := range g.files {
		f.LinkTo(main)
	}
	// A file that was the main file of its own item is a sidecar now (e.g. a JPEG
	// imported alone, then its RAW appeared): that item goes
	for _, f := range g.files[1:] {
		if old, err := itemsProxy.GetItemByGuid(f.GUID); err == nil {
			if err := itemsProxy.DeleteItem(old); err != nil {
				return nil, err
			}
			v.logger.Info("Former main file is a sidecar now", l.String("file", f.Path), l.String("main", main.Path))
		}
	}
	if _, err := filesProxy.UpdateFiles(g.files); err != nil {
		return nil, err
	}

	item, outcome, err := itemsProxy.ValidateFile(main, g.exifs[0])
	if err != nil {
		return nil, err
	}

	if outcome == model.OutcomeMoved && v.outputsComplete(item.Guid) {
		// Same content at another path: nothing to transcode or extract again
		item.State = dto.Ready
		if _, err := itemsProxy.UpdateItem(item); err != nil {
			return nil, err
		}
		return nil, chain.ErrSkippedItem
	}

	return &RawItem{Item: item, Exif: g.exifs, Files: g.files, Kinds: g.kinds}, nil
}

func (v *validator) Stop() {}
