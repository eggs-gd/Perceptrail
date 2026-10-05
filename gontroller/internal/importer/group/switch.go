// Package group: the second step of the import — the walk's files in, whole assets
// out (New), a sub-chain. Inside: one switch asks the enabled providers in order
// (library.Enabled: Apple Photos…, the plain folder last) and a file goes to the
// grouper of the first that claims it — each grouper a step of its own. Every grouper
// keeps a buffer of open groups and sends a group when it is complete; when its
// input closes (the walk ended, the switch returned) it sends what it still holds.
// A file the walk found missing goes to its provider too.
package group

import (
	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	chain "github.com/eggs-gd/go-chain"
)

// Library: what grouping asks of a library — whether a found file is its, and its
// grouper
type Library interface {
	Claims(path string) bool
	Grouper() provider.Grouper
}

// New: the sub-chain from in (the walk's files) to out (whole assets: every grouper
// writes to it, so it closes once every grouper has returned)
func New[L Library](libraries []L, in <-chan dto.WalkedFile, out chan<- dto.Asset) chain.Processor {
	grouping := chain.NewChainProcessor(nil)
	toGroupers := make([]chan<- dto.WalkedFile, len(libraries))
	for i, library := range libraries {
		ch := make(chan dto.WalkedFile)
		toGroupers[i] = ch
		grouping.AddStep(chain.NewDecorator(ch, out, library.Grouper()))
	}
	grouping.AddStep(chain.NewSwitch(in, toGroupers, Switch[L]{Libraries: libraries}))
	return grouping
}

// Switch: a file to the grouper of the first library that claims it (its index in
// Libraries)
type Switch[L Library] struct {
	Libraries []L
}

func (s Switch[L]) Switch(f dto.WalkedFile) (int, error) {
	for i, library := range s.Libraries {
		if library.Claims(f.Path) {
			return i, nil
		}
	}
	return 0, chain.ErrSkippedItem // no plain folder enabled: nobody takes it
}
