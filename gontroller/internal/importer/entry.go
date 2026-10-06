// Package importer: the import service — it builds the import chain and runs it,
// pass after pass. The chain's top has only linear steps, each in its own package (a
// sub-chain when it has several), each knowing its tools; the top knows none of them
// (roadmap "The next chains").
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
//     its input to the end, gives what it holds (a grouper its last group) and
//     returns; an output closes once every
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
	"errors"
	"time"

	"perceptrail/gontroller/internal/importer/commit"
	"perceptrail/gontroller/internal/importer/exif"
	"perceptrail/gontroller/internal/importer/gate"
	"perceptrail/gontroller/internal/importer/group"
	"perceptrail/gontroller/internal/importer/identify"
	"perceptrail/gontroller/internal/importer/walk"
	"perceptrail/gontroller/internal/library"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/perceptor"

	chain "github.com/eggs-gd/go-chain"

	l "github.com/eggs-gd/go-zap-decor"
)

// Config: what the import reads of the config — the root walked, where the
// previews go, the pause between passes, the exiftool to run
type Config interface {
	LibraryRoot() string
	CacheDir() string
	Rescan() time.Duration
	Exiftool() string
}

// Store: the model as the import's steps ask it
type Store interface {
	walk.Store
	gate.Store
	identify.Store
	exif.Items
	commit.Store
}

// Service: the import, pass after pass
type Service struct {
	cfg    Config
	db     Store
	logger *l.Logger
}

func New(cfg Config, db Store, logger *l.Logger) *Service {
	return &Service{cfg: cfg, db: db, logger: logger}
}

// Start: what changed since the last run first (identify's detection, the
// perceptors), then pass after pass, with the rescan pause between, until ctx ends
func (s *Service) Start(ctx context.Context) {
	identify.Migrate(s.db, s.logger)
	// A perceptor new or changed since the last run: its items are processed again;
	// the rows of items gone meanwhile dropped
	if err := exif.Reconcile(s.db, s.logger); err != nil {
		s.logger.Error("Perceptors' rows not checked", l.Error(err))
	}
	for ctx.Err() == nil {
		_ = s.Pass(ctx) // its errors are logged
		select {
		case <-time.After(s.cfg.Rescan()):
		case <-ctx.Done():
		}
	}
}

// Pass: one pass of the import — a new chain, run to its end (the walk, every
// group of it through every step). Its steps' errors are logged and returned
// together (skips never are: they are on purpose).
func (s *Service) Pass(ctx context.Context) error {
	errs := make(chan error)
	var all []error
	collected := make(chan struct{})
	go func() {
		for err := range errs {
			s.logger.Error("Import Error", l.Error(err))
			all = append(all, err)
		}
		close(collected)
	}()
	s.importChain(errs).Process(ctx)
	close(errs)
	<-collected
	return errors.Join(all...)
}

// importChain: one pass of the import, its channels new (a pass closes them)
func (s *Service) importChain(errs chan<- error) chain.ChainProcessor {
	// walk → group: a file's row (seen: Changed; or Missing)
	found := make(chan dto.WalkedFile)
	// group → gate: a whole asset and what is gone of the library (Missing); at the
	// end, what each grouper held
	grouped := make(chan dto.Asset)
	// gate → identify: the assets that need work
	stored := make(chan dto.Asset)
	// identify → exif: the identified items
	identified := make(chan *identify.Item)
	// exif → commit: + what the perceptors found, their values kept
	perceived := make(chan *identify.Item)

	c := chain.NewChainProcessor(errs)
	c.AddStep(walk.New(s.db, s.cfg.LibraryRoot(), library.Skipped(s.cfg.LibraryRoot()), s.logger, found))
	c.AddStep(group.New(library.Enabled(), found, grouped))
	c.AddStep(gate.New(s.db, s.logger, grouped, stored))
	c.AddStep(identify.New(s.db, s.cfg.CacheDir(), s.cfg.Exiftool(), perceptor.ExifTags(), s.logger, stored, identified))
	c.AddStep(exif.New(s.logger, identified, perceived))
	c.AddStep(commit.New(s.db, perceived))
	return c
}
