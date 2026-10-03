// Package importer: the import chain — files found, items published. Its top has
// only linear stages, each a sub-chain of its own that knows its tools; the top
// knows none of them (roadmap "Chains").
//
//	discover → identify → core → plugins → commit
//
//	discover  the groups that need work; the files table up to date — walk, group
//	          (the providers), gate (deletions after a walk)
//	identify  the item known — exiftool, kinds and the main file, the item's
//	          identity, sizes, the cheap preview
//	core      the core perceptors (date, size, …): they write into the item
//	plugins   the external perceptors (Go plugins): they only read it
//	commit    the item published: Visible (a preview) or Waiting (none), with its
//	          perceptors' values
//
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Not obvious:
//   - The walker flushes the chain after a walk: every step passes the flush on after
//     the values before it, a grouper first gives what it holds; where branches join
//     (the groupers into the gate) it passes once every branch has flushed. At the
//     gate it runs the deletions (after a complete walk that found files, never
//     under an unreadable directory); at the end of the chain it means the walk's work
//     is done — the next walk starts only then (walks never overlap).
//   - The chain is async: deletions may run before a moved file is validated, so the
//     validator restores deleted items by hash.
//   - One asset is processed again without a walk (Refresh): its provider forms its
//     group, discover sends it to its gate.
package importer

import (
	"context"
	"sync"
	"time"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/importer/commit"
	"perceptrail/gontroller/pkg/importer/core"
	"perceptrail/gontroller/pkg/importer/discover"
	"perceptrail/gontroller/pkg/importer/identify"
	external "perceptrail/gontroller/pkg/importer/plugins"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Pause between the end of one walk's processing and the next walk (config: rescan)
const defaultRescan = time.Minute

type importerService struct {
	appCtx app.AppContext

	importChain *chain.Chain

	// One asset again (Refresh): discover sends its group to the gate; whoever
	// waits hears when its item leaves the chain or the gate drops the group
	discover *discover.Stage
	waits    *assetWaits
}

func NewImporterService(ctx app.AppContext) *importerService {
	logger := ctx.Logger(string(app.LogImporter))
	db := model.NewProxy(ctx.Logger(string(app.LogDB)))
	progress := discover.NewProgress()

	// Every stage's errors (skips never get here: they are on purpose)
	errch := make(chan error)
	go func() {
		for err := range errch {
			logger.Error("Import Error", l.Error(err))
		}
	}()
	rescan := ctx.Config().Rescan
	if rescan <= 0 {
		rescan = defaultRescan
	}

	// Between the stages (each message type belongs to the stage that yields it)
	// discover → identify: the groups that need work, stored (rows of the files table)
	stored := chain.NewPipe[discover.Group](0)
	// identify → core: the identified items
	identified := chain.NewPipe[*identify.Item](0)
	// core → plugins: + what the core perceptors found
	cored := chain.NewPipe[*identify.Item](0)
	// plugins → commit: + what the external perceptors found
	perceived := chain.NewPipe[*identify.Item](0)
	// commit → the end: published items (later: events to the client); buffered so
	// the closer does not wait for the end
	items := chain.NewPipe[*dto.ItemDto](1000)

	waits := newAssetWaits()
	disc := discover.New(ctx.Config().Path, rescan, providers.Enabled(), progress, waits.done, db, plugins.Pm, stored, logger)

	importChain := chain.New(errch)
	importChain.AddStep(disc)
	importChain.AddStep(identify.New(plugins.Pm.ExifTags(), ctx.Config().CacheDir(), db, stored, identified, logger))
	importChain.AddStep(core.New(plugins.Pm.Core(), identified, cored, logger))
	importChain.AddStep(external.New(plugins.Pm.External(), cored, perceived, logger))
	importChain.AddStep(commit.New(db, plugins.Pm, perceived, items))
	// The end: an item's waiters hear it; the walk's flush here means its work is done
	importChain.AddStep(chain.Sink(items, func(it *dto.ItemDto) { waits.done(it.Guid) }, progress.Done))

	return &importerService{
		appCtx:      ctx,
		importChain: importChain,
		discover:    disc,
		waits:       waits,
	}
}

// Refresh processes one asset of a provider's library again, now — the library (Apple
// Photos…) has just made a file of it local (on demand): no walk of the whole library
// for one photo. Its group goes to the gate like any group (only what changed passes;
// deletions are not touched — they come with the walk's flush; a walk sending the
// same asset at the same time just processes it twice into the same item). Waits
// until the item has left the chain or the gate dropped the group (nothing changed:
// Photos drew from what was local), at most wait; false if neither (no such asset,
// too slow) — the next walk catches up anyway.
func (s *importerService) Refresh(uuid string, wait time.Duration) bool {
	done := s.waits.add(uuid)
	defer s.waits.drop(uuid, done)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if !s.discover.Regroup(ctx, uuid) {
		return false
	}
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// assetWaits: who waits for an asset's item to be done (Refresh) — told when the
// item leaves the chain or the gate drops its group
type assetWaits struct {
	mu sync.Mutex
	by map[string][]chan struct{}
}

func newAssetWaits() *assetWaits { return &assetWaits{by: map[string][]chan struct{}{}} }

// add: a channel closed when the asset is done
func (w *assetWaits) add(key string) chan struct{} {
	ch := make(chan struct{})
	w.mu.Lock()
	w.by[key] = append(w.by[key], ch)
	w.mu.Unlock()
	return ch
}

// drop: the waiter gave up (or heard already)
func (w *assetWaits) drop(key string, ch chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	chs := w.by[key]
	for i, c := range chs {
		if c == ch {
			w.by[key] = append(chs[:i], chs[i+1:]...)
			break
		}
	}
	if len(w.by[key]) == 0 {
		delete(w.by, key)
	}
}

// done: the asset's item is done — everyone waiting hears
func (w *assetWaits) done(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.by[key] {
		close(c)
	}
	delete(w.by, key)
}

func (s *importerService) Start(ctx context.Context) {
	s.importChain.Process(ctx)
}
