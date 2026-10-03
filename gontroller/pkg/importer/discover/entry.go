// Package discover: the first stage of the import — the groups that need work, the
// files table up to date (and what is gone deleted after a walk).
//
//	walk → group (the providers' switch, a grouper each) → gate
//
// It knows the file system, the providers and the files table; nothing after it
// does. One asset's group can be sent to the gate again without a walk (Regroup).
package discover

import (
	"context"
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

// Deps: what discover works with — the libraries' providers, the model, the
// perceptors' two calls, who hears of a keyed group the gate drops (nothing
// changed: one asset again on demand answers at once), the logger
type Deps struct {
	Providers  []providers.Provider
	DB         Store
	Perceptors Perceptors
	Dropped    func(key string) // nil: nobody
	Logger     *l.Logger
}

// Stage: the discover sub-chain, its way in for one group (Regroup), and the walk's
// end (WalkDone)
type Stage struct {
	*chain.Chain
	providers []providers.Provider
	groups    *chain.Pipe[providers.Group] // group → gate
	progress  *Progress
}

// New: root is walked again every rescan, once the previous walk's work is done
// (WalkDone); out gets the groups that need work
func New(root string, rescan time.Duration, deps Deps, out *chain.Pipe[Group]) *Stage {
	progress := NewProgress()
	// walk → group: one file (path + stat); the walk's flush
	found := chain.NewPipe[providers.Found](0)
	// group → gate: a complete group; on the flush, what each grouper held
	groups := chain.NewPipe[providers.Group](0)

	stage := chain.New(nil)
	stage.AddStep(chain.Entry(found, NewWalker(root, rescan, progress, deps.Logger)))
	stage.AddStep(group.NewGrouping(deps.Providers, found, groups))
	stage.AddStep(chain.Decorate(groups, out, NewGate(deps, progress)))
	return &Stage{Chain: stage, providers: deps.Providers, groups: groups, progress: progress}
}

// WalkDone: the walk's flush reached the end of the chain (every group of it went
// through every step) — the next walk may start after the rescan pause
func (s *Stage) WalkDone() { s.progress.Done() }

// Regroup: one asset's group formed again by its provider (the library made a file
// of it local), sent to the gate like any group — no walk; false if no provider has
// it or the gate did not take it before ctx ended
func (s *Stage) Regroup(ctx context.Context, key string) bool {
	for _, p := range s.providers {
		a, ok := p.Regroup(key)
		if !ok {
			continue
		}
		return s.groups.Send(ctx, providers.Group{Asset: a})
	}
	return false
}
