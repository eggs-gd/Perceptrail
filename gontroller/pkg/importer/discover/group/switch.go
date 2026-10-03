package group

import (
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"
)

// Package groups: the grouping sub-chain — found files in, whole assets out
// (NewGrouping), as processing is its own sub-chain after the gate. Inside: one
// switch asks the enabled providers in order (providers.Enabled: Apple Photos…, the
// plain folder last) and a file goes to the grouper of the first that claims it —
// each grouper a step of its own. Every grouper keeps a buffer of open groups and
// sends a group when it is complete.

// NewGrouping: the sub-chain from chin (found files, the end-of-walk marker) to
// chout (whole assets; a marker from every grouper — the gate waits for len(ps));
// its steps report to errch (skips: files held, not complete yet)
func NewGrouping(ps []providers.Provider, chin <-chan providers.Found, chout chan<- providers.Group, errch chan error) chain.ChainProcessor {
	grouping := chain.NewChainProcessor(errch)
	toGroupers := make([]chan<- providers.Found, len(ps))
	for i, p := range ps {
		toGrouper := make(chan providers.Found)
		toGroupers[i] = toGrouper
		grouping.AddStep(chain.NewDecorator(toGrouper, chout, p.Grouper()))
	}
	grouping.AddStep(chain.NewSwitch(chin, toGroupers, Switch{Providers: ps}))
	return grouping
}

// Switch: a file to the grouper of the first provider that claims it (its index in
// Providers); the end-of-walk marker to every grouper, so each flushes what it
// holds — the gate waits for a marker from each
type Switch struct {
	Providers []providers.Provider
}

func (s Switch) Switch(ev providers.Found) (map[int]providers.Found, error) {
	if ev.Done != nil {
		all := make(map[int]providers.Found, len(s.Providers))
		for i := range s.Providers {
			all[i] = ev
		}
		return all, nil
	}
	for i, p := range s.Providers {
		if p.Claims(ev.Entry.Path) {
			return map[int]providers.Found{i: ev}, nil
		}
	}
	return nil, chain.ErrSkippedItem // no plain folder enabled: nobody takes it
}

func (Switch) Stop() {}
