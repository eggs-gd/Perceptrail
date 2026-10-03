// Package identify: the second stage of the import — the item known: its identity,
// its metadata, its files' kinds and roles, what it can show now.
//
//	read (exiftool, in parallel) → classify (kinds, roles, the main file) → merge
//	(the metadata package) → fingerprint (the main file's bytes) → validate (the item: same / changed / moved / new /
//	broken) → embedded (a preview extracted from the main file, when nothing else
//	shows) → sizes (pixels, codecs) → pick (what to show now) → yield (the Item)
//
// What crosses its boundary is only Item: the working draft (every file, its exif,
// kinds) stays here.
//
// exiftool lives here and nowhere else (read, embedded).
package identify

import (
	"perceptrail/gontroller/pkg/importer/discover"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// Store: what identify reads and writes — its steps' needs
type Store interface {
	KindsStore
	HashStore
	ValidatorStore
	SizesStore
}

// exiftool processes and parallel readers (groups are independent)
const workers = 5

// Steps: the stage's steps' logic, in order — New runs them between channels, Run
// one group at a time (the tests). exiftool comes in through Read.Extract and
// Embedded.Extract (a fake in tests).
type Steps struct {
	Read        *Reader
	Classify    Classifier
	Merge       *Merge
	Fingerprint Fingerprint
	Validate    *Validator
	Embedded    *Embedded
	Sizes       *Sizes
	Pick        Pick
	Yield       Yield
}

// NewSteps: pool runs exiftool (nil in tests: set the Extract funcs)
func NewSteps(pool *exiftoolPool, cfg Config) *Steps {
	return &Steps{
		Read:     NewReader(pool, cfg.Tags, cfg.Logger),
		Merge:    NewMerge(cfg.Tags),
		Validate: NewValidator(cfg.DB, cfg.Logger),
		Embedded: NewEmbedded(pool, cfg.CacheDir, cfg.Logger),
		Sizes:    NewSizes(cfg.DB),
	}
}

// Run: one group through every step, as the chain runs it
func (s *Steps) Run(g discover.Group) (*Item, error) {
	d, err := s.Read.Decorate(g)
	for _, step := range []chain.Decorator[*draft, *draft]{s.Classify, s.Merge, s.Fingerprint, s.Validate, s.Embedded, s.Sizes, s.Pick} {
		if err != nil {
			return nil, err
		}
		d, err = step.Decorate(d)
	}
	if err != nil {
		return nil, err
	}
	return s.Yield.Decorate(d)
}

// Config: what identify works with — the tags the perceptors read (read from the
// files, merged into the package), where embedded previews are extracted, the
// model, the logger
type Config struct {
	Tags     []string
	CacheDir string
	DB       Store
	Logger   *l.Logger
}

// New: in — the groups that need work (stored: rows of the files table); out — the
// identified items. Its errors go to the chain it runs in.
func New(cfg Config, in *chain.Pipe[discover.Group], out *chain.Pipe[*Item]) chain.Processor {
	// The kinds' table changed since the files were judged "not media": judged again
	if err := reclassifyIgnored(cfg.DB, cfg.Logger); err != nil {
		cfg.Logger.Error("MIME version check failed", l.Error(err))
	}
	// The fingerprint changed: every item gets the new one
	if err := forgetOldHashes(cfg.DB, cfg.Logger); err != nil {
		cfg.Logger.Error("Fingerprint version check failed", l.Error(err))
	}
	s := NewSteps(newExiftoolPool(workers, cfg.Logger), cfg)

	// read → classify: the files and their metadata
	read := chain.NewPipe[*draft](0)
	// classify → merge: + kinds and roles, the main file first
	classified := chain.NewPipe[*draft](0)
	// merge → fingerprint: + the asset's metadata package
	merged := chain.NewPipe[*draft](0)
	// fingerprint → validate: + the main file's fingerprint
	fingerprinted := chain.NewPipe[*draft](0)
	// validate → embedded: + the item (its GUID); not media does not get here
	validated := chain.NewPipe[*draft](0)
	// embedded → sizes: + the extracted preview, if one was needed
	extracted := chain.NewPipe[*draft](0)
	// sizes → pick: + every file's pixels and codec (stored)
	sized := chain.NewPipe[*draft](0)
	// pick → yield: + what to show now
	picked := chain.NewPipe[*draft](0)

	stage := chain.New(nil)
	stage.AddStep(chain.Parallel(workers, in, read, s.Read)) // groups are independent: one pool
	stage.AddStep(chain.Decorate(read, classified, s.Classify))
	stage.AddStep(chain.Decorate(classified, merged, s.Merge))
	stage.AddStep(chain.Decorate(merged, fingerprinted, s.Fingerprint))
	stage.AddStep(chain.Decorate(fingerprinted, validated, s.Validate))
	stage.AddStep(chain.Decorate(validated, extracted, s.Embedded))
	stage.AddStep(chain.Decorate(extracted, sized, s.Sizes))
	stage.AddStep(chain.Decorate(sized, picked, s.Pick))
	stage.AddStep(chain.Decorate(picked, out, s.Yield))
	return stage
}
