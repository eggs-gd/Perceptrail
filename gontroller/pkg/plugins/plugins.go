// Package plugins: the registry of this run's perceptors — the core ones and the
// external Go plugins, loaded once at start (Load), and their storages. What a
// perceptor means to the import (which run there, what they read, their rows) is
// the importer's business: it reads All and Store. One per process: package
// functions, like providers.
package plugins

import (
	"fmt"
	"path/filepath"
	"plugin"
	"sync"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins/settings"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
)

// Config: what the registry needs — the external plugins' files, which perceptors
// run and which the client sees, where their storages go (the DB driver, the data
// directory), the logger
type Config struct {
	Plugins    []string
	Perceptors settings.Perceptors
	Driver     string
	DataDir    string
	Logger     *l.Logger
}

// The loaded perceptors (enabled in the config), the core ones first, and the
// storage of each that keeps data (its Schema), by perceptor name
var (
	mu         sync.RWMutex
	perceptors []api.Perceptor
	stores     map[string]*model.PerceptorStore
	config     Config
	logger     *l.Logger
	loaded     bool
)

// Load loads every perceptor once, at start: the built-in ones (core: given by the
// caller — they import this package for their contract, it does not import them),
// then the external plugins of the config; the ones switched off in the config do not
// run
func Load(cfg Config, core ...api.Perceptor) error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return nil
	}
	config, logger = cfg, cfg.Logger

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
		st, err := model.OpenPerceptorStore(config.Driver, filepath.Join(config.DataDir, "perceptors"), s)
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
