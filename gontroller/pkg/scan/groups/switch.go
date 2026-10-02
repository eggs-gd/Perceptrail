package groups

import (
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"
)

// Package groups: the switch between the found files and the groupers. Every
// enabled provider (providers.Enabled: Apple Photos…, the plain folder last) has a
// grouper of its own; a file goes to the first provider that claims it — the plain
// folder claims what nobody else did. Every grouper keeps a buffer of open groups
// and sends a group when it is complete; the files gate after them is shared.

// Switch: a file to the grouper of the first provider that claims it (its index in
// Providers); the end-of-walk marker to every grouper, so each flushes what it
// holds — the gate waits for a marker from each
type Switch struct {
	Providers []providers.Provider
}

// NewSwitch: outs[i] is the grouper of Providers[i]
func NewSwitch(ps []providers.Provider, chin <-chan flow.FileEvent, outs []chan<- flow.FileEvent) chain.Processor {
	return chain.NewSwitch(chin, outs, Switch{Providers: ps})
}

func (s Switch) Switch(ev flow.FileEvent) (map[int]flow.FileEvent, error) {
	if ev.Done != nil {
		all := make(map[int]flow.FileEvent, len(s.Providers))
		for i := range s.Providers {
			all[i] = ev
		}
		return all, nil
	}
	for i, p := range s.Providers {
		if p.Claims(ev.Entry.Path) {
			return map[int]flow.FileEvent{i: ev}, nil
		}
	}
	return nil, chain.ErrSkippedItem // no plain folder enabled: nobody takes it
}

func (Switch) Stop() {}
