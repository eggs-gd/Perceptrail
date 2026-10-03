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
	"perceptrail/gontroller/pkg/importer/gate"
	"perceptrail/gontroller/pkg/plugins"

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

// Parallel readers, and the stage's exiftool processes (groups are independent)
const workers = 5

// Option: a test's change to the stage
type Option func(*options)

type options struct{ tool Exiftool }

// WithExiftool: a test's exiftool instead of the stage's own processes
func WithExiftool(tool Exiftool) Option { return func(o *options) { o.tool = tool } }

// New: in — the groups that need work (stored: rows of the files table); out — the
// identified items; embedded previews go under cacheDir. It runs its own exiftool
// (a pool of processes, closed when its steps stop); what is read besides
// identify's own tags is what the loaded perceptors declare (plugins.ExifTags). Its
// errors go to the chain it runs in.
func New(db Store, cacheDir string, logger *l.Logger, in *chain.Pipe[gate.Group], out *chain.Pipe[*Item], opts ...Option) chain.Processor {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	tool := o.tool
	if tool == nil {
		tool = newPool(workers, logger)
	}
	// The kinds' table changed since the files were judged "not media": judged again
	if err := reclassifyIgnored(db, logger); err != nil {
		logger.Error("MIME version check failed", l.Error(err))
	}
	// The fingerprint changed: every item gets the new one
	if err := forgetOldHashes(db, logger); err != nil {
		logger.Error("Fingerprint version check failed", l.Error(err))
	}
	tags := plugins.ExifTags()

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
	stage.AddStep(chain.Parallel(workers, in, read, NewReader(tool, tags, logger))) // groups are independent
	stage.AddStep(chain.Decorate(read, classified, Classifier{}))
	stage.AddStep(chain.Decorate(classified, merged, NewMerge(tags)))
	stage.AddStep(chain.Decorate(merged, fingerprinted, Fingerprint{}))
	stage.AddStep(chain.Decorate(fingerprinted, validated, NewValidator(db, logger)))
	stage.AddStep(chain.Decorate(validated, extracted, NewEmbedded(tool, cacheDir, logger)))
	stage.AddStep(chain.Decorate(extracted, sized, NewSizes(db)))
	stage.AddStep(chain.Decorate(sized, picked, Pick{}))
	stage.AddStep(chain.Decorate(picked, out, Yield{}))
	return stage
}
