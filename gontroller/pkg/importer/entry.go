// Package importer: the import chain — files found, items published. Its top has
// only linear steps, each in its own package (a sub-chain when it has several), each
// knowing its tools; the top knows none of them (roadmap "Chains").
//
//	walk → group → gate → identify → exif → commit
//
//	walk       the library's files (path + stat), a walk per request, then a flush
//	group      whole assets: the providers' groupers (the plain folder last)
//	gate       the files table up to date; only the groups that need work pass;
//	           on a walk's flush, its deletions
//	identify   the item known — exiftool, kinds and the main file, the item's
//	           identity, sizes, the cheap preview
//	exif       the EXIF perceptors: the built-in ones write into the item (date,
//	           size, length), the external Go plugins only read it; their values kept
//	commit     the chain's end: the item published, Visible (a preview) or Waiting
//
// The service runs the chain and its cycle: a walk request, the chain Done (the
// walk's flush reached the end), the rescan pause, the next request; and what spans
// it: one asset again (Refresh, sent straight into the gate's input) and who waits
// for it.
//
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Not obvious:
//   - The walk flushes the chain: every step passes the flush on after the values
//     before it, a grouper first gives what it holds; where branches join (the
//     groupers into the gate) it passes once every branch has flushed. The gate runs
//     the walk's deletions on it (every group of the walk has passed it), the exif
//     step prunes the perceptors' rows of gone items. Once it reached commit the
//     chain is Done: then the pause, the next walk. Walks never overlap.
//   - A moved file: its old path may be deleted before the new one is identified;
//     validate restores a deleted item by fingerprint: the GUID stays.
//   - One asset again without a walk (Refresh): its provider forms its group, the top
//     sends it into the gate's input asked for (Requested: processed even if nothing
//     changed), and waits for its item at the end.
package importer

import (
	"context"
	"sync"
	"time"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/importer/commit"
	"perceptrail/gontroller/pkg/importer/exif"
	"perceptrail/gontroller/pkg/importer/gate"
	"perceptrail/gontroller/pkg/importer/group"
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/importer/walk"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Pause between the end of one walk's processing and the next walk (config: rescan)
const defaultRescan = time.Minute

type importerService struct {
	importChain *chain.Chain
	walks       *chain.Pipe[struct{}] // a walk request
	rescan      time.Duration         // the pause between the end of one walk and the next

	// One asset again (Refresh): its provider forms its group, it goes into the
	// gate's input; whoever waits hears when its item is published
	providers []providers.Provider
	grouped   *chain.Pipe[providers.Group]
	waits     *assetWaits
}

func NewImporterService(ctx app.AppContext) *importerService {
	logger := ctx.Logger(string(app.LogImporter))
	db := model.NewProxy(ctx.Logger(string(app.LogDB)))

	// Every step's errors (skips never get here: they are on purpose)
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

	// Between the steps (each message type belongs to the step that yields it)
	// the service → walk: a walk request
	walks := chain.NewPipe[struct{}](0)
	// walk → gate: the walk's result, for the deletions on its flush
	var walked walk.Result
	// walk → group: one file (path + stat); the walk's flush
	found := chain.NewPipe[dto.ItemEntry](0)
	// group → gate: a whole asset; on the flush, what each grouper held. Refresh
	// sends an asset asked for here too
	grouped := chain.NewPipe[providers.Group](0)
	// gate → identify: the groups that need work, stored (rows of the files table)
	stored := chain.NewPipe[gate.Group](0)
	// identify → exif: the identified items
	identified := chain.NewPipe[*identify.Item](0)
	// exif → commit: + what the perceptors found, their values kept
	perceived := chain.NewPipe[*identify.Item](0)

	ps := providers.Enabled()
	waits := newAssetWaits()

	importChain := chain.New(errch)
	importChain.AddStep(walk.New(ctx.Config().Path, &walked, logger, walks, found))
	importChain.AddStep(group.New(ps, found, grouped))
	importChain.AddStep(gate.New(db, logger, &walked, grouped, stored))
	importChain.AddStep(identify.New(db, ctx.Config().CacheDir(), logger, stored, identified))
	importChain.AddStep(exif.New(db, identified, perceived, logger))
	importChain.AddStep(commit.New(db, waits.done, perceived))

	return &importerService{
		importChain: importChain,
		walks:       walks,
		rescan:      rescan,
		providers:   ps,
		grouped:     grouped,
		waits:       waits,
	}
}

// Refresh processes one asset of a provider's library again, now — the library (Apple
// Photos…) has just made a file of it local (on demand): no walk of the whole library
// for one photo. Its provider forms its group, it goes into the gate's input asked for
// (processed even if nothing changed: Photos may have drawn from what was local);
// deletions are not touched (they come after a walk). Waits until its item has left
// the chain, at most wait; false if not (no such asset, too slow) — the next walk
// catches up anyway.
func (s *importerService) Refresh(uuid string, wait time.Duration) bool {
	done := s.waits.add(uuid)
	defer s.waits.drop(uuid, done)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	for _, p := range s.providers {
		a, ok := p.Regroup(uuid)
		if !ok {
			continue
		}
		if !s.grouped.Send(ctx, providers.Group{Asset: a, Requested: true}) {
			return false
		}
		select {
		case <-done:
			return true
		case <-ctx.Done():
			return false
		}
	}
	return false
}

// assetWaits: who waits for an asset's item to be done (Refresh) — told when the
// item leaves the chain
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

// Start: the chain runs; walk after walk — a request, the chain Done, the rescan
// pause — until ctx ends
func (s *importerService) Start(ctx context.Context) {
	stopped := make(chan struct{})
	go func() {
		s.importChain.Process(ctx)
		close(stopped)
	}()
	for s.walks.Send(ctx, struct{}{}) {
		select {
		case <-s.importChain.Done():
		case <-ctx.Done():
		}
		select {
		case <-time.After(s.rescan):
		case <-ctx.Done():
		}
	}
	<-stopped
}
