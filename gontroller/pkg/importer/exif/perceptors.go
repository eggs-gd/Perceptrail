package exif

import (
	"fmt"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
)

// The perceptors as the import sees them: the ones that run in the chain (EXIF
// data), what they read, their rows. The registry (pkg/plugins) only knows what is
// loaded and where each one keeps its data.

// importPerceptors: the perceptors the import chain runs (EXIF data), in order
func importPerceptors() []api.Perceptor {
	var out []api.Perceptor
	for _, p := range plugins.All() {
		if p.DataProvider() == api.ExifDataProvider {
			out = append(out, p)
		}
	}
	return out
}

// corePerceptors: the built-in ones (they write into the item)
func corePerceptors() []plugins.ExifCorePerceptor {
	var out []plugins.ExifCorePerceptor
	for _, p := range importPerceptors() {
		if c, ok := p.(plugins.ExifCorePerceptor); ok {
			out = append(out, c)
		}
	}
	return out
}

// externalPerceptors: the Go plugins (they only read it); one that is not an
// api.ExifPerceptor is not run
func externalPerceptors() []api.ExifPerceptor {
	var out []api.ExifPerceptor
	for _, p := range importPerceptors() {
		if _, core := p.(plugins.ExifCorePerceptor); core {
			continue
		}
		if e, ok := p.(api.ExifPerceptor); ok {
			out = append(out, e)
		}
	}
	return out
}

// importStores: their storages — every processed item gets a row in each (a value,
// or "nothing found"); the rows of gone items are pruned
func importStores() []*model.PerceptorStore {
	var out []*model.PerceptorStore
	for _, p := range importPerceptors() {
		if st, ok := plugins.Store(p.Name()); ok {
			out = append(out, st)
		}
	}
	return out
}

// saveValues: a row in each import perceptor's storage for the item — its value, or
// "processed, nothing found" (no GPS); values gives a storage's value (keep)
func saveValues(guid string, values func(store string) (api.Values, bool)) error {
	for _, st := range importStores() {
		v, _ := values(st.Name())
		if err := st.Save(guid, v); err != nil {
			return fmt.Errorf("perceptor %s: %w", st.Name(), err)
		}
	}
	return nil
}

// Items: what the perceptors' bookkeeping asks of the model — every item, and the
// ones to process again
type Items interface {
	GetAllGuids() ([]string, error)
	MarkRework(guids []string) (int64, error)
}

// MarkUnprocessed runs at start: an item an import perceptor has no row for (the
// perceptor is new, or its schema changed: its storage was recreated) is marked for
// rework — the gate sends its group once more, its files unchanged. Once at start,
// not per group per walk: a perceptor's rows change only with the perceptors.
func MarkUnprocessed(db Items, logger *l.Logger) error {
	guids, err := db.GetAllGuids()
	if err != nil {
		return err
	}
	missing := map[string]bool{}
	for _, st := range importStores() {
		has, err := st.Guids()
		if err != nil {
			return fmt.Errorf("perceptor %s: %w", st.Name(), err)
		}
		done := make(map[string]bool, len(has))
		for _, g := range has {
			done[g] = true
		}
		for _, g := range guids {
			if !done[g] {
				missing[g] = true
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	rework := make([]string, 0, len(missing))
	for g := range missing {
		rework = append(rework, g)
	}
	n, err := db.MarkRework(rework)
	logger.Info("Perceptors without a row: items processed again", l.Int("items", int(n)))
	return err
}

// Prune drops the import perceptors' rows of items that are gone — at the end of a
// pass (the gate has done the walk's deletions by then). Other perceptors' rows are
// the maintenance chain's (roadmap)
func Prune(db Items, logger *l.Logger) {
	guids, err := db.GetAllGuids()
	if err != nil {
		logger.Error("Perceptor storage: can't read items", l.Error(err))
		return
	}
	keep := make(map[string]bool, len(guids))
	for _, g := range guids {
		keep[g] = true
	}
	for _, st := range importStores() {
		n, err := st.Prune(func(guid string) bool { return keep[guid] })
		if err != nil {
			logger.Error("Perceptor storage: prune failed", l.String("store", st.Name()), l.Error(err))
		} else if n > 0 {
			logger.Info("Perceptor storage: pruned", l.String("store", st.Name()), l.Int("rows", n))
		}
	}
}
