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

type pluginManager struct {
	ctx       app.AppContext
	logger    *l.Logger
	pluginsMu sync.RWMutex
	plugins   []api.Perceptor
	loaded    bool
	// The storage of each perceptor that keeps data (its Schema), by perceptor name
	stores map[string]*model.PerceptorStore
}

var Pm *pluginManager = &pluginManager{}

// GetPlugins returns loaded plugins list. Thread-safe.
func (pm *pluginManager) GetPlugins() []api.Perceptor {
	pm.pluginsMu.RLock()
	defer pm.pluginsMu.RUnlock()

	result := make([]api.Perceptor, len(pm.plugins))
	copy(result, pm.plugins)
	return result
}

// LoadPlugins loads all plugins once at startup
func (pm *pluginManager) LoadPlugins(ctx app.AppContext) error {
	pm.ctx = ctx
	pm.logger = ctx.Logger(string(app.LogPlugins))

	pm.pluginsMu.Lock()
	defer pm.pluginsMu.Unlock()

	if pm.loaded {
		return nil
	}

	// Load core plugins first
	corePlugins := []api.Perceptor{
		date.Perceptor,
		size.Perceptor,
		duration.Perceptor,
	}

	// Load external plugins
	externalPlugins, err := pm.loadExternalPlugins()
	if err != nil {
		pm.logger.Error("failed to load external plugins:", l.Error(err))
		return err
	}

	// Combine all plugins; the ones switched off in the config do not run
	cfg := ctx.Config().Perceptors
	pm.plugins = nil
	for _, p := range append(corePlugins, externalPlugins...) {
		if !cfg.Enabled(p.Name()) {
			pm.logger.Info("Perceptor disabled", l.String("name", p.Name()))
			continue
		}
		pm.plugins = append(pm.plugins, p)
	}
	pm.loaded = true
	pm.openStores()

	pm.logger.Info("Loaded plugins", l.Any("core", len(corePlugins)), l.Any("external", len(externalPlugins)), l.Any("total", len(pm.plugins)))

	return nil
}

// openStores: a storage per perceptor that declares data — data_dir/perceptors/
// (SQLite: a file each). One that cannot be opened is logged: its perceptor runs,
// its values are not kept.
func (pm *pluginManager) openStores() {
	pm.stores = map[string]*model.PerceptorStore{}
	cfg := pm.ctx.Config()
	for _, p := range pm.plugins {
		s := p.Schema()
		if s.Store == "" {
			continue
		}
		st, err := model.OpenPerceptorStore(cfg.Database.Driver, filepath.Join(cfg.DataDir, "perceptors"), s)
		if err != nil {
			pm.logger.Error("Perceptor storage not opened", l.String("perceptor", p.Name()), l.Error(err))
			continue
		}
		pm.stores[p.Name()] = st
	}
}

// ImportStores: the storages of the perceptors that run in the import chain (EXIF
// data): every processed item gets a row in each (a value, or "nothing found")
func (pm *pluginManager) ImportStores() []*model.PerceptorStore {
	var out []*model.PerceptorStore
	for _, p := range pm.GetPlugins() {
		if st, ok := pm.stores[p.Name()]; ok && p.DataProvider() == api.ExifDataProvider {
			out = append(out, st)
		}
	}
	return out
}

// Stores: every perceptor storage (pruning gone items)
func (pm *pluginManager) Stores() []*model.PerceptorStore {
	var out []*model.PerceptorStore
	for _, st := range pm.stores {
		out = append(out, st)
	}
	return out
}

// Core: the import chain's core perceptors (built in, EXIF data), in order
func (pm *pluginManager) Core() []exif_core.ExifCorePerceptor {
	var out []exif_core.ExifCorePerceptor
	for _, p := range pm.GetPlugins() {
		if c, ok := p.(exif_core.ExifCorePerceptor); ok && p.DataProvider() == api.ExifDataProvider {
			out = append(out, c)
		}
	}
	return out
}

// External: the import chain's external perceptors (Go plugins, EXIF data), in order
func (pm *pluginManager) External() []api.ExifPerceptor {
	var out []api.ExifPerceptor
	for _, p := range pm.GetPlugins() {
		if _, core := p.(exif_core.ExifCorePerceptor); core || p.DataProvider() != api.ExifDataProvider {
			continue
		}
		e, ok := p.(api.ExifPerceptor)
		if !ok {
			pm.logger.Error("EXIF plugin has no NewProcessor, skipped", l.String("plugin", p.Name()))
			continue
		}
		out = append(out, e)
	}
	return out
}

// ExifTags: every tag the import chain's perceptors read (ExifTagger), once each —
// the only ones read from the files besides the core's own
func (pm *pluginManager) ExifTags() []string {
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
	for _, p := range pm.Core() {
		add(p.ExifTags())
	}
	for _, p := range pm.External() {
		add(p.ExifTags())
	}
	return out
}

// Unprocessed: an import perceptor has no row for the item (new, or its schema
// changed): the item goes through the import once more
func (pm *pluginManager) Unprocessed(guid string) bool {
	for _, st := range pm.ImportStores() {
		if has, err := st.Has(guid); err == nil && !has {
			return true
		}
	}
	return false
}

// SaveValues: a row in each import perceptor's storage for the item — its value, or
// "processed, nothing found" (no GPS); values gives a storage's value
func (pm *pluginManager) SaveValues(guid string, values func(store string) (api.Values, bool)) error {
	for _, st := range pm.ImportStores() {
		v, _ := values(st.Name())
		if err := st.Save(guid, v); err != nil {
			return fmt.Errorf("perceptor %s: %w", st.Name(), err)
		}
	}
	return nil
}

// Prune: every perceptor's rows of items not kept (gone)
func (pm *pluginManager) Prune(keep func(guid string) bool) {
	for _, st := range pm.Stores() {
		n, err := st.Prune(keep)
		if err != nil {
			pm.logger.Error("Perceptor storage: prune failed", l.String("store", st.Name()), l.Error(err))
		} else if n > 0 {
			pm.logger.Info("Perceptor storage: pruned", l.String("store", st.Name()), l.Int("rows", n))
		}
	}
}

// LoadValues: a perceptor's values for these items (none if it keeps nothing)
func (pm *pluginManager) LoadValues(perceptor string, guids []string) (string, map[string]api.Values, error) {
	st, ok := pm.stores[perceptor]
	if !ok {
		return "", nil, nil
	}
	v, err := st.Load(guids)
	return st.Name(), v, err
}

// ClientPerceptors: the loaded perceptors whose view the client is given (config
// `client`), core first — the first is the gallery's default view
func (pm *pluginManager) ClientPerceptors() []api.Perceptor {
	var out []api.Perceptor
	for _, p := range pm.GetPlugins() {
		if pm.ctx.Config().Perceptors.Client(p.Name()) {
			out = append(out, p)
		}
	}
	return out
}

func (pm *pluginManager) loadExternalPlugins() ([]api.Perceptor, error) {
	var result []api.Perceptor

	for _, file := range pm.ctx.Config().Plugins {
		p, err := loadPlugin(file)
		if err != nil {
			pm.logger.Error("Failed to load plugin", l.Any("file", file), l.Error(err))
			continue
		}
		result = append(result, p)
		pm.logger.Info("Loaded external plugin", l.Any("name", p.Name()), l.Any("type", p))
	}

	return result, nil
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
