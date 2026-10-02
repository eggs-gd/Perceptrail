// Package importer: the import chain — files found, items published. Its top has
// only linear stages, each a sub-chain of its own that knows its tools; the top
// knows none of them (roadmap "Chains").
//
//	discover → identify → perceive
//
//	discover  the groups that need work; the files table up to date — walk, group
//	          (the providers), gate (deletions after a walk)
//	identify  the item known — exiftool, kinds and the main file, the item's
//	          identity, the cheap preview
//	perceive  the perceptors (core, plugins), then the item published: Visible
//	          (a preview) or Waiting (none)
//
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Not obvious:
//   - Two error channels: discover reports to the walk's, the stages after the gate
//     to the processing one — every group the gate lets through ends there or as an
//     item, and the progress counts them; the next walk starts only when all are done
//     (walks never overlap).
//   - The end-of-walk marker goes through the groupers to the gate: deletions run
//     only when every grouper's marker reached it, after a complete walk that found
//     files, never under an unreadable directory.
//   - The chain is async: deletions may run before a moved file is validated, so the
//     validator restores deleted items by hash.
//   - One asset is processed again without a walk (Refresh): its provider forms its
//     group, discover sends it to its gate.
package importer

import (
	"context"
	"errors"
	"sync"
	"time"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/importer/discover"
	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/importer/perceive"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Pause between the end of one walk's processing and the next walk (config: rescan)
const defaultRescan = time.Minute

type importerService struct {
	appCtx app.AppContext

	errch    chan error
	items    chan *dto.ItemDto
	progress *flow.Progress

	importChain chain.ChainProcessor

	// One asset again (Refresh): discover sends its group to the gate; whoever
	// waits hears when its item leaves the chain or the gate drops the group
	discover *discover.Stage
	waits    *assetWaits
}

func NewImporterService(ctx app.AppContext) *importerService {
	logger := ctx.Logger(string(app.LogImporter))
	db := model.NewProxy(ctx.Logger(string(app.LogDB)))

	logErr := func(err error) {
		if !errors.Is(err, chain.ErrSkippedItem) { // skips are on purpose (buffered, unchanged, not media)
			logger.Error("Import Error", l.Error(err))
		}
	}
	progress := flow.NewProgress()

	// discover's errors (the walk, the groupers, the gate)
	errch := make(chan error)
	go func() {
		for err := range errch {
			logErr(err)
		}
	}()
	// After the gate every group ends as an item (items) or here: both count as done
	errProcessing := make(chan error)
	go func() {
		for err := range errProcessing {
			progress.Finished()
			logErr(err)
		}
	}()
	rescan := ctx.Config().Rescan
	if rescan <= 0 {
		rescan = defaultRescan
	}

	// Between the stages (the message types: flow)
	// discover → identify: the groups that need work, stored (rows of the files table)
	stored := make(chan flow.FileGroup)
	// identify → perceive: the identified items
	identified := make(chan *flow.RawItem)
	// perceive → nobody yet: published items, drained in Start (later: events to the
	// client); buffered so the closer does not wait for the drain
	items := make(chan *dto.ItemDto, 1000)

	waits := newAssetWaits()
	disc := discover.New(ctx.Config().Path, rescan, providers.Enabled(), progress, waits.done, db, stored, errch, logger)

	importChain := chain.NewChainProcessor(errch)
	importChain.AddStep(disc)
	importChain.AddStep(identify.New(ctx.Config().CacheDir(), db, stored, identified, errProcessing, logger))
	importChain.AddStep(perceive.New(db, identified, items, errProcessing, logger))

	return &importerService{
		appCtx:      ctx,
		errch:       errch,
		items:       items,
		progress:    progress,
		importChain: importChain,
		discover:    disc,
		waits:       waits,
	}
}

// Refresh processes one asset of a provider's library again, now — the library (Apple
// Photos…) has just made a file of it local (on demand): no walk of the whole library
// for one photo. Its group goes to the gate like any group (only what changed passes; deletions are not
// touched — they come with the walk's marker; a walk sending the same asset at the
// same time just processes it twice into the same item). Waits until the item has
// left the chain or the gate dropped the group (nothing changed: Photos drew from
// what was local), at most wait; false if neither (no such asset, too slow) — the
// next walk catches up anyway.
func (s *importerService) Refresh(uuid string, wait time.Duration) bool {
	done := s.waits.add(uuid)
	defer s.waits.drop(uuid, done)

	timeout := time.After(wait)
	if !s.discover.Regroup(uuid, timeout) {
		return false
	}
	select {
	case <-done:
		return true
	case <-timeout:
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

func (s *importerService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Nothing consumes finished items yet (later: events to the client); drain them,
	// or the closer blocks once the buffer is full
	go func() {
		for {
			select {
			case it := <-s.items:
				s.progress.Finished()
				s.waits.done(it.Guid)
			case <-ctx.Done():
				return
			}
		}
	}()

	s.importChain.Process(ctx)

	<-ctx.Done()
}
