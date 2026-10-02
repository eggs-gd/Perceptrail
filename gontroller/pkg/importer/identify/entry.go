// Package identify: the second stage of the import — the item known: its identity,
// its metadata, its files' kinds and roles, what it can show now.
//
//	read (exiftool, in parallel) → classify (kinds, the main file) → validate (the
//	item: same / changed / moved / new / broken) → preview (what to show now)
//
// exiftool lives here and nowhere else.
package identify

import (
	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// exiftool processes and parallel read steps (groups are independent)
const workers = 5

// New: in — the groups that need work (stored: rows of the files table); out — the
// identified items; previews are extracted under cacheDir. Its steps report to
// errch (every group ends here or as an item: the progress counts them).
func New(cacheDir string, db model.Store, in <-chan flow.FileGroup, out chan<- *flow.RawItem, errch chan error, logger *l.Logger) chain.ChainProcessor {
	// The kinds' table changed since the files were judged "not media": judged again
	if err := reclassifyIgnored(db, db, logger); err != nil {
		logger.Error("MIME version check failed", l.Error(err))
	}

	// read → classify: the files and their metadata
	read := make(chan *flow.RawItem)
	// classify → validate: + kinds, the main file first
	classified := make(chan *flow.RawItem)
	// validate → preview: + the item (its GUID); not media does not get here
	validated := make(chan *flow.RawItem)

	exiftool := newExiftoolPool(workers, logger)
	reader := NewReader(exiftool, logger)
	stage := chain.NewChainProcessor(errch)
	for range workers { // N readers on the same channels, sharing the pool
		stage.AddStep(chain.NewDecorator(in, read, reader))
	}
	stage.AddStep(chain.NewDecorator(read, classified, Classifier{}))
	stage.AddStep(chain.NewDecorator(classified, validated, NewValidator(db, logger)))
	stage.AddStep(chain.NewDecorator(validated, out, NewPreview(db, exiftool, cacheDir, logger)))
	return stage
}
