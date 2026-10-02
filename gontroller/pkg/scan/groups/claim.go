package groups

import (
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"
)

// Package groups turns found files into whole assets (flow.FileGroup). The
// providers (providers.Enabled) are steps one after another: a file a provider
// claims goes to its own grouper, the rest on to the next; the generic grouper
// (generic) takes what nobody claimed — a plain folder. Every grouper keeps a
// buffer of open groups and sends a group when it is complete; the files gate after
// them is shared.

// Claim outputs: the provider's own grouper, the next step
const (
	Own = iota
	Next
)

// Claim: a provider's step of the chain
type Claim struct {
	Provider providers.Provider
}

// NewClaim: what p claims to own (its grouper), the rest to next
func NewClaim(p providers.Provider, chin <-chan flow.FileEvent, own, next chan<- flow.FileEvent) chain.Processor {
	return chain.NewSwitch(chin, []chan<- flow.FileEvent{Own: own, Next: next}, Claim{Provider: p})
}

// Switch: a claimed file to the provider's grouper, any other on; the end-of-walk
// marker to both — every grouper flushes what it holds, and the steps after get it
func (c Claim) Switch(ev flow.FileEvent) (map[int]flow.FileEvent, error) {
	if ev.Done != nil {
		return map[int]flow.FileEvent{Own: ev, Next: ev}, nil
	}
	if c.Provider.Claims(ev.Entry.Path) {
		return map[int]flow.FileEvent{Own: ev}, nil
	}
	return map[int]flow.FileEvent{Next: ev}, nil
}

func (Claim) Stop() {}

// Branches: how many groupers send an end-of-walk marker to the gate — one per
// provider and the generic one
func Branches(ps []providers.Provider) int { return len(ps) + 1 }
