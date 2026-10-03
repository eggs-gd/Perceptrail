// Package importer: the import chain — files found, items published. Its top has
// only linear steps, each in its own package (a sub-chain when it has several), each
// knowing its tools; the top knows none of them (roadmap "Chains").
//
//	walk → group → gate → identify → exif_core → exif_ext → commit
//
//	walk       the library's files (path + stat), one walk when asked; then a flush
//	group      whole assets: the providers' groupers (the plain folder last)
//	gate       the files table up to date; only the groups that need work pass
//	identify   the item known — exiftool, kinds and the main file, the item's
//	           identity, sizes, the cheap preview
//	exif_core  the built-in EXIF perceptors (date, size, length): they write into it
//	exif_ext   the external EXIF perceptors (Go plugins): they only read it
//	commit     the item published: Visible (a preview) or Waiting (none), with its
//	           perceptors' values
//
// What spans the steps is the top's: the walk cycle (cycle.go), one asset again
// (Refresh), the perceptors' bookkeeping (perceptors.go).
//
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Not obvious:
//   - The walker flushes the chain after a walk: every step passes the flush on after
//     the values before it, a grouper first gives what it holds; where branches join
//     (the groupers into the gate) it passes once every branch has flushed. At the end
//     of the chain it means the walk's work is done: then the deletions, the
//     perceptors' rows of gone items, the pause, the next walk (cycle.go) — walks
//     never overlap.
//   - Deletions come after the walk's groups went through the whole chain; a moved
//     file was validated before its old path is deleted, and validate restores a
//     deleted item by fingerprint anyway: the GUID stays.
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
	"perceptrail/gontroller/pkg/importer/exif_core"
	"perceptrail/gontroller/pkg/importer/exif_ext"
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
	walker      *walk.Walker

	// One asset again (Refresh): its provider forms its group, it goes into the
	// gate's input; whoever waits hears when its item leaves the chain
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
	// A perceptor new or changed since the last run: its items are processed again
	if err := markUnprocessed(db, logger); err != nil {
		logger.Error("Perceptors' rows not checked", l.Error(err))
	}

	// Between the steps (each message type belongs to the step that yields it)
	// walk → group: one file (path + stat); the walk's flush
	found := chain.NewPipe[dto.ItemEntry](0)
	// group → gate: a whole asset; on the flush, what each grouper held. Refresh
	// sends an asset asked for here too
	grouped := chain.NewPipe[providers.Group](0)
	// gate → identify: the groups that need work, stored (rows of the files table)
	stored := chain.NewPipe[gate.Group](0)
	// identify → exif_core: the identified items
	identified := chain.NewPipe[*identify.Item](0)
	// exif_core → exif_ext: + what the core perceptors found
	cored := chain.NewPipe[*identify.Item](0)
	// exif_ext → commit: + what the external perceptors found
	perceived := chain.NewPipe[*identify.Item](0)
	// commit → the end: published items (later: events to the client); buffered so
	// the closer does not wait for the end
	items := chain.NewPipe[*dto.ItemDto](1000)

	ps := providers.Enabled()
	walker := walk.New(ctx.Config().Path, logger)
	waits := newAssetWaits()
	c := &cycle{db: db, walker: walker, rescan: rescan, logger: logger}

	importChain := chain.New(errch)
	importChain.AddStep(chain.Entry(found, walker))
	importChain.AddStep(group.New(ps, found, grouped))
	importChain.AddStep(gate.New(db, logger, grouped, stored))
	importChain.AddStep(identify.New(identify.Config{Tags: exifTags(), CacheDir: ctx.Config().CacheDir(), DB: db, Logger: logger}, stored, identified))
	importChain.AddStep(exif_core.New(corePerceptors(), identified, cored, logger))
	importChain.AddStep(exif_ext.New(externalPerceptors(), cored, perceived, logger))
	importChain.AddStep(commit.New(db, saveValues, perceived, items))
	// The end: an item's waiters hear it; the walk's flush here means its work is
	// done — the cycle goes on
	importChain.AddStep(chain.Sink(items, func(it *dto.ItemDto) { waits.done(it.Guid) }, c.walked))

	return &importerService{
		importChain: importChain,
		walker:      walker,
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

func (s *importerService) Start(ctx context.Context) {
	s.walker.Next() // the first walk; the cycle asks for the next ones
	s.importChain.Process(ctx)
}
