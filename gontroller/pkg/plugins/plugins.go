package plugins

import (
	"fmt"
	"plugin"
	"sync"

	"perceptrail/gontroller/pkg/app"
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

	// Combine all plugins
	pm.plugins = append(corePlugins, externalPlugins...)
	pm.loaded = true

	pm.logger.Info("Loaded plugins", l.Any("core", len(corePlugins)), l.Any("external", len(externalPlugins)), l.Any("total", len(pm.plugins)))

	return nil
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
