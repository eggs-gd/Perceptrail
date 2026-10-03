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

// exiftool processes and parallel read steps (groups are independent)
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

// NewSteps: pool runs exiftool (nil in tests: set the Extract funcs); tags are what
// the perceptors read (ExifTagger: read from the files, merged into the package);
// previews are extracted under cacheDir
func NewSteps(pool *exiftoolPool, tags []string, cacheDir string, db Store, logger *l.Logger) *Steps {
	return &Steps{
		Read:     NewReader(pool, tags, logger),
		Merge:    NewMerge(tags),
		Validate: NewValidator(db, logger),
		Embedded: NewEmbedded(pool, cacheDir, logger),
		Sizes:    NewSizes(db),
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

// New: in — the groups that need work (stored: rows of the files table); out — the
// identified items; tags — what the perceptors read; previews are extracted under
// cacheDir. Its steps report to
// errch (every group ends here or as an item: the progress counts them).
func New(tags []string, cacheDir string, db Store, in <-chan discover.Group, out chan<- *Item, errch chan error, logger *l.Logger) chain.ChainProcessor {
	// The kinds' table changed since the files were judged "not media": judged again
	if err := reclassifyIgnored(db, logger); err != nil {
		logger.Error("MIME version check failed", l.Error(err))
	}
	// The fingerprint changed: every item gets the new one
	if err := forgetOldHashes(db, logger); err != nil {
		logger.Error("Fingerprint version check failed", l.Error(err))
	}
	s := NewSteps(newExiftoolPool(workers, logger), tags, cacheDir, db, logger)

	// read → classify: the files and their metadata
	read := make(chan *draft)
	// classify → merge: + kinds and roles, the main file first
	classified := make(chan *draft)
	// merge → fingerprint: + the asset's metadata package
	merged := make(chan *draft)
	// fingerprint → validate: + the main file's fingerprint
	fingerprinted := make(chan *draft)
	// validate → embedded: + the item (its GUID); not media does not get here
	validated := make(chan *draft)
	// embedded → sizes: + the extracted preview, if one was needed
	extracted := make(chan *draft)
	// sizes → pick: + every file's pixels and codec (stored)
	sized := make(chan *draft)
	// pick → yield: + what to show now
	picked := make(chan *draft)

	stage := chain.NewChainProcessor(errch)
	for range workers { // N readers on the same channels, sharing the pool
		stage.AddStep(chain.NewDecorator(in, read, s.Read))
	}
	stage.AddStep(chain.NewDecorator(read, classified, s.Classify))
	stage.AddStep(chain.NewDecorator(classified, merged, s.Merge))
	stage.AddStep(chain.NewDecorator(merged, fingerprinted, s.Fingerprint))
	stage.AddStep(chain.NewDecorator(fingerprinted, validated, s.Validate))
	stage.AddStep(chain.NewDecorator(validated, extracted, s.Embedded))
	stage.AddStep(chain.NewDecorator(extracted, sized, s.Sizes))
	stage.AddStep(chain.NewDecorator(sized, picked, s.Pick))
	stage.AddStep(chain.NewDecorator(picked, out, s.Yield))
	return stage
}
