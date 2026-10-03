package importer

import (
	"fmt"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
)

// The perceptors as the import sees them: the ones that run in the chain (EXIF
// data), what they read, their rows. The registry (pkg/plugins) only knows what is
// loaded and where each one keeps its data.

// corePerceptors: the import chain's core perceptors (built in, EXIF data), in order
func corePerceptors() []exif_core.ExifCorePerceptor {
	var out []exif_core.ExifCorePerceptor
	for _, p := range plugins.All() {
		if c, ok := p.(exif_core.ExifCorePerceptor); ok && p.DataProvider() == api.ExifDataProvider {
			out = append(out, c)
		}
	}
	return out
}

// externalPerceptors: the import chain's external perceptors (Go plugins, EXIF
// data), in order; one that is not an api.ExifPerceptor is not run
func externalPerceptors() []api.ExifPerceptor {
	var out []api.ExifPerceptor
	for _, p := range plugins.All() {
		if _, core := p.(exif_core.ExifCorePerceptor); core || p.DataProvider() != api.ExifDataProvider {
			continue
		}
		if e, ok := p.(api.ExifPerceptor); ok {
			out = append(out, e)
		}
	}
	return out
}

// exifTags: every tag the import chain's perceptors read (ExifTagger), once each —
// the only ones read from the files besides identify's own
func exifTags() []string {
	seen := map[string]bool{}
	var out []string
	add := func(tags []string) {
		for _, t := range tags {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	for _, p := range corePerceptors() {
		add(p.ExifTags())
	}
	for _, p := range externalPerceptors() {
		add(p.ExifTags())
	}
	return out
}

// importStores: the storages of the perceptors that run in the import chain: every
// processed item gets a row in each (a value, or "nothing found")
func importStores() []*model.PerceptorStore {
	var out []*model.PerceptorStore
	for _, p := range plugins.All() {
		if st, ok := plugins.Store(p.Name()); ok && p.DataProvider() == api.ExifDataProvider {
			out = append(out, st)
		}
	}
	return out
}

// unprocessed: an import perceptor has no row for the item (new, or its schema
// changed): the item goes through the import once more (the gate)
func unprocessed(guid string) bool {
	for _, st := range importStores() {
		if has, err := st.Has(guid); err == nil && !has {
			return true
		}
	}
	return false
}

// saveValues: a row in each import perceptor's storage for the item — its value, or
// "processed, nothing found" (no GPS); values gives a storage's value (commit)
func saveValues(guid string, values func(store string) (api.Values, bool)) error {
	for _, st := range importStores() {
		v, _ := values(st.Name())
		if err := st.Save(guid, v); err != nil {
			return fmt.Errorf("perceptor %s: %w", st.Name(), err)
		}
	}
	return nil
}

// pruner: drops every perceptor's rows of items not kept (gone) — the deletions after
// a walk (discover's sweep)
func pruner(logger *l.Logger) func(keep func(guid string) bool) {
	return func(keep func(guid string) bool) {
		for _, p := range plugins.All() {
			st, ok := plugins.Store(p.Name())
			if !ok {
				continue
			}
			n, err := st.Prune(keep)
			if err != nil {
				logger.Error("Perceptor storage: prune failed", l.String("store", st.Name()), l.Error(err))
			} else if n > 0 {
				logger.Info("Perceptor storage: pruned", l.String("store", st.Name()), l.Int("rows", n))
			}
		}
	}
}
