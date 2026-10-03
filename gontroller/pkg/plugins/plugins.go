// Package plugins: the perceptors of this run — the core ones and the external Go
// plugins, loaded once at start (Load), with their storages. One per process:
// package functions, like providers.
package plugins

import (
	"fmt"
	"path/filepath"
	"plugin"
	"sync"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins/exif_core"
	"perceptrail/gontroller/pkg/plugins/exif_core/date"
	"perceptrail/gontroller/pkg/plugins/exif_core/duration"
	"perceptrail/gontroller/pkg/plugins/exif_core/size"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
)

// The loaded perceptors (enabled in the config), the core ones first, and the
// storage of each that keeps data (its Schema), by perceptor name
var (
	mu         sync.RWMutex
	perceptors []api.Perceptor
	stores     map[string]*model.PerceptorStore
	config     *app.Config
	logger     *l.Logger
	loaded     bool
)

// Load loads every perceptor once, at start: the core ones, then the external
// plugins of the config; the ones switched off in the config do not run
func Load(ctx app.AppContext) error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return nil
	}
	config = ctx.Config()
	logger = ctx.Logger(string(app.LogPlugins))

	core := []api.Perceptor{date.Perceptor, size.Perceptor, duration.Perceptor}
	external := loadExternal()
	perceptors = nil
	for _, p := range append(core, external...) {
		if !config.Perceptors.Enabled(p.Name()) {
			logger.Info("Perceptor disabled", l.String("name", p.Name()))
			continue
		}
		perceptors = append(perceptors, p)
	}
	loaded = true
	openStores()

	logger.Info("Loaded plugins", l.Any("core", len(core)), l.Any("external", len(external)), l.Any("total", len(perceptors)))
	return nil
}

// All: the loaded perceptors, in order
func All() []api.Perceptor {
	mu.RLock()
	defer mu.RUnlock()
	return append([]api.Perceptor(nil), perceptors...)
}

// openStores: a storage per perceptor that declares data — data_dir/perceptors/
// (SQLite: a file each). One that cannot be opened is logged: its perceptor runs,
// its values are not kept.
func openStores() {
	stores = map[string]*model.PerceptorStore{}
	for _, p := range perceptors {
		s := p.Schema()
		if s.Store == "" {
			continue
		}
		st, err := model.OpenPerceptorStore(config.Database.Driver, filepath.Join(config.DataDir, "perceptors"), s)
		if err != nil {
			logger.Error("Perceptor storage not opened", l.String("perceptor", p.Name()), l.Error(err))
			continue
		}
		stores[p.Name()] = st
	}
}

// importStores: the storages of the perceptors that run in the import chain (EXIF
// data): every processed item gets a row in each (a value, or "nothing found")
func importStores() []*model.PerceptorStore {
	var out []*model.PerceptorStore
	for _, p := range All() {
		if st, ok := stores[p.Name()]; ok && p.DataProvider() == api.ExifDataProvider {
			out = append(out, st)
		}
	}
	return out
}

// Core: the import chain's core perceptors (built in, EXIF data), in order
func Core() []exif_core.ExifCorePerceptor {
	var out []exif_core.ExifCorePerceptor
	for _, p := range All() {
		if c, ok := p.(exif_core.ExifCorePerceptor); ok && p.DataProvider() == api.ExifDataProvider {
			out = append(out, c)
		}
	}
	return out
}

// External: the import chain's external perceptors (Go plugins, EXIF data), in order
func External() []api.ExifPerceptor {
	var out []api.ExifPerceptor
	for _, p := range All() {
		if _, core := p.(exif_core.ExifCorePerceptor); core || p.DataProvider() != api.ExifDataProvider {
			continue
		}
		e, ok := p.(api.ExifPerceptor)
		if !ok {
			logger.Error("EXIF plugin is not an ExifPerceptor (ExifTags, Decorator), skipped", l.String("plugin", p.Name()))
			continue
		}
		out = append(out, e)
	}
	return out
}

// ExifTags: every tag the import chain's perceptors read (ExifTagger), once each —
// the only ones read from the files besides the core's own
func ExifTags() []string {
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
	for _, p := range Core() {
		add(p.ExifTags())
	}
	for _, p := range External() {
		add(p.ExifTags())
	}
	return out
}

// Unprocessed: an import perceptor has no row for the item (new, or its schema
// changed): the item goes through the import once more
func Unprocessed(guid string) bool {
	for _, st := range importStores() {
		if has, err := st.Has(guid); err == nil && !has {
			return true
		}
	}
	return false
}

// SaveValues: a row in each import perceptor's storage for the item — its value, or
// "processed, nothing found" (no GPS); values gives a storage's value
func SaveValues(guid string, values func(store string) (api.Values, bool)) error {
	for _, st := range importStores() {
		v, _ := values(st.Name())
		if err := st.Save(guid, v); err != nil {
			return fmt.Errorf("perceptor %s: %w", st.Name(), err)
		}
	}
	return nil
}

// Prune: every perceptor's rows of items not kept (gone)
func Prune(keep func(guid string) bool) {
	for _, st := range stores {
		n, err := st.Prune(keep)
		if err != nil {
			logger.Error("Perceptor storage: prune failed", l.String("store", st.Name()), l.Error(err))
		} else if n > 0 {
			logger.Info("Perceptor storage: pruned", l.String("store", st.Name()), l.Int("rows", n))
		}
	}
}

// LoadValues: a perceptor's values for these items (none if it keeps nothing)
func LoadValues(perceptor string, guids []string) (string, map[string]api.Values, error) {
	st, ok := stores[perceptor]
	if !ok {
		return "", nil, nil
	}
	v, err := st.Load(guids)
	return st.Name(), v, err
}

// Client: the loaded perceptors whose view the client is given (config `client`),
// core first — the first is the gallery's default view
func Client() []api.Perceptor {
	var out []api.Perceptor
	for _, p := range All() {
		if config.Perceptors.Client(p.Name()) {
			out = append(out, p)
		}
	}
	return out
}

// loadExternal: the plugins of the config; one that cannot be loaded is logged
func loadExternal() []api.Perceptor {
	var out []api.Perceptor
	for _, file := range config.Plugins {
		p, err := loadPlugin(file)
		if err != nil {
			logger.Error("Failed to load plugin", l.Any("file", file), l.Error(err))
			continue
		}
		out = append(out, p)
		logger.Info("Loaded external plugin", l.Any("name", p.Name()), l.Any("type", p))
	}
	return out
}

func loadPlugin(path string) (api.Perceptor, error) {
	plug, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symPlugin, err := plug.Lookup("Perceptor")
	if err != nil {
		return nil, fmt.Errorf("failed to find 'Perceptor' symbol: %w", err)
	}

	var perceptor *api.Perceptor
	perceptor, ok := symPlugin.(*api.Perceptor)
	if !ok {
		return nil, fmt.Errorf("invalid plugin type: want %T, got %T", perceptor, symPlugin)
	}

	return *perceptor, nil
}
