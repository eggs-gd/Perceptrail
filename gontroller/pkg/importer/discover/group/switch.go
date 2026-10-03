package group

import (
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"
)

// Package group: the grouping sub-chain — found files in, whole assets out
// (NewGrouping). Inside: one switch asks the enabled providers in order
// (providers.Enabled: Apple Photos…, the plain folder last) and a file goes to the
// grouper of the first that claims it — each grouper a step of its own. Every grouper
// keeps a buffer of open groups and sends a group when it is complete; on the walk's
// flush (the chain gives it to every grouper) it sends what it still holds.

// NewGrouping: the sub-chain from in (found files) to out (whole assets: every
// grouper writes to it, so a flush passes on once every grouper has flushed)
func NewGrouping(ps []providers.Provider, in *chain.Pipe[providers.Found], out *chain.Pipe[providers.Group]) chain.Processor {
	grouping := chain.New(nil)
	toGroupers := make([]*chain.Pipe[providers.Found], len(ps))
	for i, p := range ps {
		toGroupers[i] = chain.NewPipe[providers.Found](0)
		grouping.AddStep(chain.Decorate(toGroupers[i], out, p.Grouper()))
	}
	grouping.AddStep(chain.Route(in, toGroupers, Switch{Providers: ps}))
	return grouping
}

// Switch: a file to the grouper of the first provider that claims it (its index in
// Providers)
type Switch struct {
	Providers []providers.Provider
}

func (s Switch) Route(ev providers.Found) (int, error) {
	for i, p := range s.Providers {
		if p.Claims(ev.Entry.Path) {
			return i, nil
		}
	}
	return 0, chain.ErrSkippedItem // no plain folder enabled: nobody takes it
}
