// Package apple will group an Apple Photos library by its database (assets:
// original = the source; render, derivatives, Live Photo video = linked to it):
// the first file of a library loads the asset links, a group goes out when all
// its files have arrived. Not implemented: groups.appleEnabled keeps files away
// from it; it only passes the end-of-walk marker on.
package apple

import (
	"errors"

	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"
)

type Grouper struct{}

func NewGrouper(chin <-chan flow.FileEvent, chout chan<- flow.FileGroup) chain.Processor {
	return chain.NewDecorator(chin, chout, Grouper{})
}

var errNotImplemented = errors.New("apple photos grouper: not implemented yet")

func (Grouper) Decorate(ev flow.FileEvent) (flow.FileGroup, error) {
	if ev.Done != nil {
		return flow.FileGroup{Done: ev.Done}, nil
	}
	return flow.FileGroup{}, errNotImplemented
}

func (Grouper) Stop() {}
