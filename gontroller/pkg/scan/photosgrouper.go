package scan

import (
	"errors"

	"github.com/eggs-gd/perceplib/chain"
)

// photosGrouper will group an Apple Photos library by its database (assets:
// original, render, Live Photo video). Not implemented: photosLibraryEnabled
// keeps files away from it; it only passes the marker on.
type photosGrouper struct{}

func NewPhotosGrouper(chin <-chan fileEvent, chout chan<- FileGroup) chain.Processor {
	return chain.NewDecorator(chin, chout, photosGrouper{})
}

var errPhotosNotImplemented = errors.New("apple photos grouper: not implemented yet")

func (photosGrouper) Decorate(ev fileEvent) (FileGroup, error) {
	if ev.done != nil {
		return FileGroup{Done: ev.done}, nil
	}
	return FileGroup{}, errPhotosNotImplemented
}

func (photosGrouper) Stop() {}
