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
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// Group: what discover yields — one whole asset that needs work, in our format: its
// files are rows of the files table (GUIDs, links), the main file first when its
// source knows it. What the source said about it rides along.
type Group struct {
	Files []*dto.FileDto
	// The item's GUID when the source knows the asset (Apple Photos: its UUID; then
	// Files[0] is the main file and is not re-ranked); "": a plain folder's group
	Key string
	// What to show first, best first (stored rows); nil: identify decides
	Show []*dto.FileDto
	// The source's own metadata (exiftool's tag names): wins over the files' EXIF;
	// MetaHash is saved with the item (the gate compares it)
	Meta     api.RawExif
	MetaHash string
	// What the asset is (dto.Kind*), when the source says it
	Kind string
}

// Stage: the discover sub-chain, and its way in for one group (Regroup)
type Stage struct {
	chain.ChainProcessor
	providers []providers.Provider
	groups    chan providers.Group // group → gate
}

// New: root is walked again every rescan after the previous walk's work is done
// (progress); out gets the groups that need work; dropped hears of a keyed group
// the gate let not through (nothing changed). Its steps report to errch.
func New(root string, rescan time.Duration, ps []providers.Provider, progress *Progress,
	dropped func(key string), db Store, perceptors Perceptors, out chan<- Group, errch chan error, logger *l.Logger) *Stage {

	// walk → group: one file (path + stat), or the end-of-walk marker
	found := make(chan providers.Found)
	// group → gate: a complete group, and/or a grouper's marker (one per provider)
	groups := make(chan providers.Group)

	stage := chain.NewChainProcessor(errch)
	stage.AddStep(NewFsWalker(root, rescan, progress, found, logger))
	stage.AddStep(group.NewGrouping(ps, found, groups, errch))
	stage.AddStep(chain.NewDecorator(groups, out, NewGate(db, perceptors, len(ps), progress, dropped, logger)))
	return &Stage{ChainProcessor: stage, providers: ps, groups: groups}
}

// Regroup: one asset's group formed again by its provider (the library made a file
// of it local), sent to the gate like any group — no walk; false if no provider has
// it or the gate did not take it before timeout
func (s *Stage) Regroup(key string, timeout <-chan time.Time) bool {
	for _, p := range s.providers {
		a, ok := p.Regroup(key)
		if !ok {
			continue
		}
		select {
		case s.groups <- providers.Group{Asset: a}:
			return true
		case <-timeout:
			return false
		}
	}
	return false
}
