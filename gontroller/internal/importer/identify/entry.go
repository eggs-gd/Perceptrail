// Package identify: the stage of the import that knows the item — its identity, its
// metadata, its files' kinds and roles, what it can show now. Three steps, at the
// real boundaries:
//
//	read      in parallel, no DB: the metadata (one exiftool call per group), the
//	          kinds and the main file (classify), the metadata package (merge), the
//	          main file's fingerprint
//	validate  one at a time, the DB: not media / broken (ignored), else the item —
//	          same / changed / moved / new
//	show      every file's pixels and codecs (stored), what to show now (pick; the
//	          preview embedded in the main file as the last resort), the Item out
//
// What crosses its boundary is only Item: the working draft (every file, its exif,
// kinds) stays here. exiftool lives here and nowhere else (read, show's embedded
// preview); its processes start with the first read and close when show stops.
package identify

import (
	"perceptrail/gontroller/internal/model/dto"

	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
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

// New: in — the assets that need work; out — the identified items; embedded
// previews go under cacheDir. It runs its own exiftool (a pool of processes, closed
// when its steps stop); tags: what is read besides identify's own (what the
// perceptors declare). Its errors go to the chain it runs in.
func New(db Store, cacheDir, exiftool string, tags []string, logger *l.Logger, in <-chan dto.Asset, out chan<- *Item) chain.Processor {
	tool := newPool(workers, exiftool, logger)

	// read → validate: + metadata, kinds and the main file, the package, the
	// fingerprint
	read := make(chan *draft)
	// validate → show: + the item (its GUID); not media does not get here
	validated := make(chan *draft)

	stage := chain.NewChainProcessor(nil)
	stage.AddStep(chain.NewDecoratorN(in, read, newReader(tool, tags, logger), workers)) // groups are independent
	stage.AddStep(chain.NewDecorator(read, validated, &validate{db: db, logger: logger}))
	stage.AddStep(chain.NewDecorator(validated, out, newShow(db, tool, cacheDir, logger)))
	return stage
}
