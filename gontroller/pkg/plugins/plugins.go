package plugins

import (
	"log"
	"path/filepath"
	"plugin"

	"perceptrail/perceptors"
)

func LoadPlugins() []perceptors.Perceptor {
	var plugins []perceptors.Perceptor

	files, err := filepath.Glob("plugins/*.so")
	if err != nil {
		log.Printf("Failed to read plugins directory: %v", err)
		return plugins
	}

	for _, file := range files {
		plug, err := plugin.Open(file)
		if err != nil {
			log.Printf("Failed to load plugin %s: %v", file, err)
			continue
		}

		symPlugin, err := plug.Lookup("Perceptor")
		if err != nil {
			log.Printf("Failed to find 'Plugin' symbol in %s: %v", file, err)
			continue
		}

		perceptorInstance, ok := symPlugin.(perceptors.Perceptor)
		if !ok {
			log.Printf("Invalid plugin type in %s", file)
			continue
		}

		plugins = append(plugins, perceptorInstance)
	}

	return plugins
}
