package plugins

import (
	"fmt"
	"log"
	"path/filepath"
	"plugin"
	"sync"

	"perceptrail/api"
	"perceptrail/gontroller/pkg/plugins/exif_core/date"
	"perceptrail/gontroller/pkg/plugins/exif_core/size"
)

var (
	pluginsMu sync.RWMutex
	plugins   []api.Perceptor
	loaded    bool
)

// GetPlugins returns loaded plugins list. Thread-safe.
func GetPlugins() []api.Perceptor {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()

	result := make([]api.Perceptor, len(plugins))
	copy(result, plugins)
	return result
}

// LoadPlugins loads all plugins once at startup
func LoadPlugins(logger *log.Logger) error {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	if loaded {
		return nil
	}

	// Load core plugins first
	corePlugins := []api.Perceptor{
		date.NewPerceptor(),
		size.NewPerceptor(),
	}

	// Load external plugins
	externalPlugins, err := loadExternalPlugins(logger)
	if err != nil {
		return fmt.Errorf("failed to load external plugins: %w", err)
	}

	// Combine all plugins
	plugins = append(corePlugins, externalPlugins...)
	loaded = true

	logger.Printf("Loaded plugins: %d core, %d external, total %d",
		len(corePlugins), len(externalPlugins), len(plugins))

	return nil
}

func loadExternalPlugins(logger *log.Logger) ([]api.Perceptor, error) {
	var result []api.Perceptor

	files, err := filepath.Glob("plugins/*.so")
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, file := range files {
		p, err := loadPlugin(file)
		if err != nil {
			logger.Printf("Failed to load plugin %s: %v", file, err)
			continue
		}
		result = append(result, p)
		logger.Printf("Loaded external plugin %s of type %T", p.Name(), p)
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

	perceptor, ok := symPlugin.(api.Perceptor)
	if !ok {
		return nil, fmt.Errorf("invalid plugin type: %T", symPlugin)
	}

	return perceptor, nil
}
