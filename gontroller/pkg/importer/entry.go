// Package importer: the import service — it builds the import chain and runs it,
// pass after pass. The chain's top has only linear steps, each in its own package (a
// sub-chain when it has several), each knowing its tools; the top knows none of them
// (roadmap "Chains").
//
//	walk → group → gate → identify → exif → commit
//
//	walk       the chain's entry: the library's files, their rows written as it goes
//	           (stat, seen now); after the walk the rows it says are gone, then a flush
//	group      whole assets: the providers' groupers (the plain folder last); a gone
//	           file goes to its provider too
//	gate       only the groups that need work pass; a gone file: the model deletes it
//	identify   the item known — exiftool, kinds and the main file, the item's
//	           identity, sizes, the cheap preview
//	exif       the EXIF perceptors: the built-in ones write into the item (date,
//	           size, length), the external Go plugins only read it; their values kept
//	commit     the chain's end: the item published, Visible (a preview) or Waiting
//
// The service runs the passes: a pass (Chain.Run: the walk, every group of it
// through every step, its flush at the end), the rescan pause, the next pass.
// Refresh marks one asset's item for the next pass.
//
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Not obvious:
//   - The walk flushes the chain: every step passes the flush on after the values
//     before it, a grouper first gives what it holds; where branches join (the
//     groupers into the gate) it passes once every branch has flushed; the exif step
//     prunes the perceptors' rows of gone items on it. Once it reached commit the pass
//     is over. Passes never overlap.
//   - The gone files come after every file the walk saw, but a moved file's old path
//     may be deleted before its new one is identified: validate restores a deleted
//     item by fingerprint, the GUID stays.
//   - Refresh does not wait: the client guesses meanwhile (the tile's cloud goes once
//     it got the rendition), the next pass makes it true; the mark does not touch
//     updated_at, so no delta brings the item back in its old state.
package importer

import (
	"context"
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

// Pause between the end of one pass and the next (config: rescan)
const defaultRescan = time.Minute

type importerService struct {
	importChain *chain.Chain
	rescan      time.Duration // the pause between the end of one pass and the next
	db          model.ItemsApi
	logger      *l.Logger
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
	// walk → group: a file's row (seen: Changed; or Gone); the walk's flush
	found := chain.NewPipe[*dto.FileDto](0)
	// group → gate: a whole asset (a gone file: its own); on the flush, what each
	// grouper held
	grouped := chain.NewPipe[providers.Asset](0)
	// gate → identify: the groups that need work
	stored := chain.NewPipe[gate.Group](0)
	// identify → exif: the identified items
	identified := chain.NewPipe[*identify.Item](0)
	// exif → commit: + what the perceptors found, their values kept
	perceived := chain.NewPipe[*identify.Item](0)

	importChain := chain.New(errch)
	importChain.AddStep(walk.New(ctx.Config().Path, db, logger, found))
	importChain.AddStep(group.New(providers.Enabled(), found, grouped))
	importChain.AddStep(gate.New(db, logger, grouped, stored))
	importChain.AddStep(identify.New(db, ctx.Config().CacheDir(), logger, stored, identified))
	importChain.AddStep(exif.New(db, identified, perceived, logger))
	importChain.AddStep(commit.New(db, perceived))

	return &importerService{importChain: importChain, rescan: rescan, db: db, logger: logger}
}

// Refresh marks one asset's item to be processed again on the next pass — the
// library (Apple Photos…) has just made a file of it local (on demand), or drawn
// from what was local: processed even if no file changed
func (s *importerService) Refresh(key string) {
	if _, err := s.db.MarkRework([]string{key}); err != nil {
		s.logger.Error("Refresh: item not marked", l.String("guid", key), l.Error(err))
	}
}

// Start: the chain runs; pass after pass — the walk through the whole chain, then
// the rescan pause — until ctx ends
func (s *importerService) Start(ctx context.Context) {
	stopped := make(chan struct{})
	go func() {
		s.importChain.Process(ctx)
		close(stopped)
	}()
	for s.importChain.Run(ctx) {
		select {
		case <-time.After(s.rescan):
		case <-ctx.Done():
		}
	}
	<-stopped
}
