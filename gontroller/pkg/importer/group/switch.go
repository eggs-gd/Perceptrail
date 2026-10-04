// Package group: the second step of the import — the walk's files in, whole assets
// out (New), a sub-chain. Inside: one switch asks the enabled providers in order
// (providers.Enabled: Apple Photos…, the plain folder last) and a file goes to the
// grouper of the first that claims it — each grouper a step of its own. Every grouper
// keeps a buffer of open groups and sends a group when it is complete; when its
// input closes (the walk ended, the switch returned) it sends what it still holds.
// A file the walk says is gone goes to its provider too.
package group

import (
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"
)

// New: the sub-chain from in (the walk's files) to out (whole assets: every grouper
// writes to it, so it closes once every grouper has returned)
func New(ps []providers.Provider, in <-chan *dto.FileDto, out chan<- dto.Asset) chain.Processor {
	grouping := chain.NewChainProcessor(nil)
	toGroupers := make([]chan<- *dto.FileDto, len(ps))
	for i, p := range ps {
		ch := make(chan *dto.FileDto)
		toGroupers[i] = ch
		grouping.AddStep(chain.NewDecorator(ch, out, p.Grouper()))
	}
	grouping.AddStep(chain.NewSwitch(in, toGroupers, Switch{Providers: ps}))
	return grouping
}

// Switch: a file to the grouper of the first provider that claims it (its index in
// Providers)
type Switch struct {
	Providers []providers.Provider
}

func (s Switch) Switch(f *dto.FileDto) (int, error) {
	for i, p := range s.Providers {
		if p.Claims(f.Path) {
			return i, nil
		}
	}
	return 0, chain.ErrSkippedItem // no plain folder enabled: nobody takes it
}
