// Package importer: the import service — it builds the import chain and runs it,
// pass after pass. The chain's top has only linear steps, each in its own package (a
// sub-chain when it has several), each knowing its tools; the top knows none of them
// (roadmap "Chains").
//
//	walk → group → gate → identify → exif → commit
//
//	walk       the chain's entry: the library's files, their rows written as it goes
//	           (stat, seen now); after the walk the rows it says are gone
//	group      whole assets: the providers' groupers (the plain folder last); a gone
//	           file goes to its provider too
//	gate       only the groups that need work pass; a gone file: the model deletes it
//	identify   the item known — exiftool, kinds and the main file, the item's
//	           identity, sizes, the cheap preview
//	exif       the EXIF perceptors: the built-in ones write into the item (date,
//	           size, length), the external Go plugins only read it; their values kept
//	commit     the chain's end: the item published, Visible (a preview) or Waiting
//
// The service runs the passes: a pass is a new chain run to its end (the walk ends,
// its output closes, each step ends after its input), then the rescan pause.
//
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Not obvious:
//   - A pass ends from its input: the walk returns, its output closes; a step reads
//     its input to the end, gives what it holds (a grouper its last group; the exif
//     step prunes the rows of gone items) and returns; an output closes once every
//     step writing to it has returned (the groupers into the gate). Process returns
//     when commit has. Passes never overlap.
//   - The gone files come after every file the walk saw, but a moved file's old path
//     may be deleted before its new one is identified: validate restores a deleted
//     item by fingerprint, the GUID stays.
//   - A library that made a file local on demand (Apple Photos) marks the item for
//     rework itself; the next pass processes it. No waiting: the client guesses
//     meanwhile; the mark does not touch updated_at, so no delta brings the item
//     back in its old state.
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

// store: the model as the import's steps ask it
type store interface {
	walk.Store
	gate.Store
	identify.Store
	exif.Items
	commit.Store
}

type importerService struct {
	db       store
	logger   *l.Logger
	errch    chan error
	root     string
	cacheDir string
	rescan   time.Duration // the pause between the end of one pass and the next
}

func NewImporterService(ctx app.AppContext) *importerService {
	rescan := ctx.Config().Rescan
	if rescan <= 0 {
		rescan = defaultRescan
	}
	return &importerService{db: model.NewProxy(ctx.Logger(app.LogDB)), logger: ctx.Logger(app.LogImporter),
		errch: make(chan error), root: ctx.Config().Path, cacheDir: ctx.Config().CacheDir(), rescan: rescan}
}

// importChain: one pass of the import, its channels new (a pass closes them)
func (s *importerService) importChain() chain.ChainProcessor {
	// walk → group: a file's row (seen: Changed; or Gone)
	found := make(chan *dto.FileDto)
	// group → gate: a whole asset (a gone file: its own); at the end, what each
	// grouper held
	grouped := make(chan dto.Asset)
	// gate → identify: the assets that need work
	stored := make(chan dto.Asset)
	// identify → exif: the identified items
	identified := make(chan *identify.Item)
	// exif → commit: + what the perceptors found, their values kept
	perceived := make(chan *identify.Item)

	c := chain.NewChainProcessor(s.errch)
	c.AddStep(walk.New(s.db, s.root, s.logger, found))
	c.AddStep(group.New(providers.Enabled(), found, grouped))
	c.AddStep(gate.New(s.db, s.logger, grouped, stored))
	c.AddStep(identify.New(s.db, s.cacheDir, s.logger, stored, identified))
	c.AddStep(exif.New(s.db, s.logger, identified, perceived))
	c.AddStep(commit.New(s.db, perceived))
	return c
}

// Start: what changed since the last run first (identify's detection, the
// perceptors), then pass after pass — a new chain each time, run to its end, then
// the rescan pause — until ctx ends
func (s *importerService) Start(ctx context.Context) {
	// Every step's errors (skips never get here: they are on purpose)
	go func() {
		for err := range s.errch {
			s.logger.Error("Import Error", l.Error(err))
		}
	}()
	identify.Migrate(s.db, s.logger)
	// A perceptor new or changed since the last run: its items are processed again
	if err := exif.MarkUnprocessed(s.db, s.logger); err != nil {
		s.logger.Error("Perceptors' rows not checked", l.Error(err))
	}
	for ctx.Err() == nil {
		s.importChain().Process(ctx)
		select {
		case <-time.After(s.rescan):
		case <-ctx.Done():
		}
	}
}
