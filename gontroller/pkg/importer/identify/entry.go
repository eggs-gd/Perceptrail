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
	"perceptrail/gontroller/pkg/model/dto"
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

// Migrate: at start, what changed in identify since the last run — the kinds'
// table (files judged "not media" are judged again), the fingerprint (every item
// gets the new one)
func Migrate(db Store, logger *l.Logger) {
	if err := reclassifyIgnored(db, logger); err != nil {
		logger.Error("MIME version check failed", l.Error(err))
	}
	if err := forgetOldHashes(db, logger); err != nil {
		logger.Error("Fingerprint version check failed", l.Error(err))
	}
}

// New: in — the assets that need work; out — the identified items; embedded previews go under cacheDir. It runs its own exiftool
// (a pool of processes, closed when its steps stop); what is read besides
// identify's own tags is what the loaded perceptors declare (plugins.ExifTags). Its
// errors go to the chain it runs in.
func New(db Store, cacheDir string, logger *l.Logger, in <-chan dto.Asset, out chan<- *Item) chain.Processor {
	tool := newPool(workers, logger)
	tags := plugins.ExifTags()

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
