// Package discover: the first stage of the import — the groups that need work, the
// files table up to date (and what is gone deleted after a walk).
//
//	walk → group (the providers' switch, a grouper each) → gate
//
// It knows the file system, the providers and the files table; nothing after it
// does. One asset's group can be sent to the gate again without a walk (Regroup).
package discover

import (
	"time"

	"perceptrail/gontroller/pkg/importer/discover/group"
	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// Stage: the discover sub-chain, and its way in for one group (Regroup)
type Stage struct {
	chain.ChainProcessor
	providers []providers.Provider
	groups    chan flow.FileGroup // group → gate
}

// New: root is walked again every rescan after the previous walk's work is done
// (progress); out gets the groups that need work; dropped hears of a keyed group
// the gate let not through (nothing changed). Its steps report to errch.
func New(root string, rescan time.Duration, ps []providers.Provider, progress *flow.Progress,
	dropped func(key string), db model.Store, out chan<- flow.FileGroup, errch chan error, logger *l.Logger) *Stage {

	// walk → group: one file (path + stat), or the end-of-walk marker
	found := make(chan flow.FileEvent)
	// group → gate: a complete group, and/or a grouper's marker (one per provider)
	groups := make(chan flow.FileGroup)

	stage := chain.NewChainProcessor(errch)
	stage.AddStep(NewFsWalker(root, rescan, progress, found, logger))
	stage.AddStep(group.NewGrouping(ps, found, groups, errch))
	stage.AddStep(chain.NewDecorator(groups, out, NewGate(db, len(ps), progress, dropped, logger)))
	return &Stage{ChainProcessor: stage, providers: ps, groups: groups}
}

// Regroup: one asset's group formed again by its provider (the library made a file
// of it local), sent to the gate like any group — no walk; false if no provider has
// it or the gate did not take it before timeout
func (s *Stage) Regroup(key string, timeout <-chan time.Time) bool {
	for _, p := range s.providers {
		g, ok := p.Regroup(key)
		if !ok {
			continue
		}
		select {
		case s.groups <- g:
			return true
		case <-timeout:
			return false
		}
	}
	return false
}
