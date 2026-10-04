// Package library: the libraries of this run — the providers enabled, in the order
// the import's switch asks them (the plain folder last), which one an item belongs
// to (on demand), and their background work. The contract they implement is package
// provider.
package library

import (
	"context"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/config"
	"perceptrail/gontroller/pkg/library/apple"
	"perceptrail/gontroller/pkg/library/apple/photokit"
	"perceptrail/gontroller/pkg/library/folder"
	"perceptrail/gontroller/pkg/library/provider"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
)

// Config: what the libraries read of the config — the root walked, which providers
// are enabled
type Config interface {
	LibraryRoot() string
	Providers() config.Providers
}

var enabled []provider.Provider

// Enable: the libraries of this run, from the config, in the order of the chain:
// Apple Photos (its library's DB and files; PhotoKit on demand, macOS), the plain
// folder last (it takes what nobody claimed). Once, at start.
func Enable(cfg Config, db apple.Items, logger *l.Logger) error {
	var libraries []provider.Provider
	if cfg.Providers().Enabled("apple") {
		libraries = append(libraries, apple.New(cfg.LibraryRoot(), photokit.Library{}, db, logger.Named("apple")))
	}
	libraries = append(libraries, folder.New())
	Use(libraries...)
	return nil
}

// Use: these libraries for the run, as given (Enable's, or a test's own)
func Use(libraries ...provider.Provider) { enabled = libraries }

// Enabled: the libraries in the chain, in order
func Enabled() []provider.Provider { return enabled }

// Of: the library an item belongs to, nil for a plain folder's
func Of(item *dto.ItemDto) provider.Provider {
	for _, lib := range enabled {
		if lib.Owns(item) {
			return lib
		}
	}
	return nil
}

// Service: the libraries' background work (access, what nothing shows yet), run
// with the server's services until they stop
func Service() app.Service { return service{} }

type service struct{}

func (service) Start(ctx context.Context) {
	for _, lib := range enabled {
		lib.Start(ctx)
	}
	<-ctx.Done()
}
