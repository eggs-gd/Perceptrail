// Package perceptor: the registry of this run's perceptors — the core ones and the
// external Go plugins, loaded once at start (Load), and their storages. What a
// perceptor means to the import (which run there, what they read, their rows) is
// the importer's business: it reads All and Store. One per process: package
// functions, like library.
package perceptor

import (
	"fmt"
	"path/filepath"
	"plugin"
	"sync"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/perceptor/date"
	"perceptrail/gontroller/internal/perceptor/duration"
	"perceptrail/gontroller/internal/perceptor/size"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
)

// Config: what the registry reads of the config — the external plugins' files,
// which perceptors run and which the client sees, where their storages go (the
// database's driver, the data directory)
type Config interface {
	Plugins() []string
	Perceptors() config.Perceptors
	Database() config.Database
	DataDir() string
}

// builtin: the perceptors built into the server (they write into the item), in
// the order they run
var builtin = []api.Perceptor{date.Perceptor, size.Perceptor, duration.Perceptor}

// The loaded perceptors (enabled in the config), the core ones first, and the
// storage of each that keeps data (its Schema), by perceptor name
var (
	mu         sync.RWMutex
	perceptors []api.Perceptor
	stores     map[string]*model.PerceptorStore
	cfg        Config
	logger     *l.Logger
	loaded     bool
)

// Load loads every perceptor once, at start: the built-in ones, then the external
// plugins of the config; the ones switched off in the config do not run
func Load(config Config, log *l.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return nil
	}
	cfg, logger = config, log

	external := loadExternal()
	perceptors = nil
	for _, p := range append(append([]api.Perceptor(nil), builtin...), external...) {
		if !cfg.Perceptors().Enabled(p.Name()) {
			logger.Info("Perceptor disabled", l.String("name", p.Name()))
			continue
		}
		perceptors = append(perceptors, p)
	}
	loaded = true
	openStores()

	logger.Info("Perceptors loaded", l.Int("built in", len(builtin)), l.Int("external", len(external)), l.Int("running", len(perceptors)))
	return nil
}

// All: the loaded perceptors, in order
func All() []api.Perceptor {
	mu.RLock()
	defer mu.RUnlock()
	return append([]api.Perceptor(nil), perceptors...)
}

// Close: every perceptor's storage closed (the end of the run)
func Close() {
	mu.Lock()
	defer mu.Unlock()
	for name, st := range stores {
		if err := st.Close(); err != nil {
			logger.Error("Perceptor storage not closed", l.String("perceptor", name), l.Error(err))
		}
	}
	stores = nil
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
		st, err := model.OpenPerceptorStore(cfg.Database().Driver, filepath.Join(cfg.DataDir(), "perceptors"), s)
		if err != nil {
			logger.Error("Perceptor storage not opened", l.String("perceptor", p.Name()), l.Error(err))
			continue
		}
		stores[p.Name()] = st
	}
}

// ExifTags: every tag the loaded perceptors read (api.ExifTagger), once each
func ExifTags() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range All() {
		t, ok := p.(api.ExifTagger)
		if !ok {
			continue
		}
		for _, tag := range t.ExifTags() {
			if !seen[tag] {
				seen[tag] = true
				out = append(out, tag)
			}
		}
	}
	return out
}

// Store: the storage of a perceptor that keeps data (its Schema); false: it keeps
// nothing, or it could not be opened
func Store(perceptor string) (*model.PerceptorStore, bool) {
	mu.RLock()
	defer mu.RUnlock()
	st, ok := stores[perceptor]
	return st, ok
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
		if cfg.Perceptors().Client(p.Name()) {
			out = append(out, p)
		}
	}
	return out
}

// loadExternal: the plugins of the config; one that cannot be loaded is logged
func loadExternal() []api.Perceptor {
	var out []api.Perceptor
	for _, file := range cfg.Plugins() {
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
