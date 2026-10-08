// Package library: the libraries of this run — the providers enabled, in the order
// the import's switch asks them (the plain folder last), which one an item belongs
// to (on demand), and their background work. The contract they implement is package
// provider.
package library

import (
	"context"

	"perceptrail/gontroller/internal/app"
	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/library/apple"
	"perceptrail/gontroller/internal/library/apple/photokit"
	"perceptrail/gontroller/internal/library/folder"
	"perceptrail/gontroller/internal/library/immich"
	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
)

// Config: what the libraries read of the config — the roots walked, which providers
// are enabled
type Config interface {
	LibraryRoots() []string
	Providers() config.Providers
}

var enabled []provider.Provider

type service struct{}

// Enable: the libraries of this run, from the config, in the order of the chain:
// Apple Photos (its library's DB and files; PhotoKit on demand, macOS), an Immich
// server (its API: when its url is set), the plain folder last (it takes what
// nobody claimed). Once, at start: an Immich set up without a key stops the start.
func Enable(cfg Config, db apple.Items, logger *l.Logger) error {
	var libraries []provider.Provider
	if cfg.Providers().Enabled("apple") {
		libraries = append(libraries, apple.New(cfg.LibraryRoots(), photokit.Library{}, db, logger.Named("apple")))
	}
	if server := cfg.Providers()["immich"]; server.URL != "" && cfg.Providers().Enabled("immich") {
		key, err := immich.Key(server.APIKeyFile)
		if err != nil {
			return err
		}
		lib, err := immich.New(server.URL, key, logger.Named("immich"))
		if err != nil {
			return err
		}
		libraries = append(libraries, lib)
	}
	libraries = append(libraries, folder.New())
	enabled = libraries
	return nil
}

// Enabled: the libraries in the chain, in order
func Enabled() []provider.Provider { return enabled }

// Skipped: the directories under the roots no enabled library wants walked (they
// hold none of their media), every library's
func Skipped(roots []string) []string {
	var skipped []string
	for _, root := range roots {
		for _, lib := range enabled {
			skipped = append(skipped, lib.Skipped(root)...)
		}
	}
	return skipped
}

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

func (service) Start(ctx context.Context) {
	for _, lib := range enabled {
		lib.Start(ctx)
	}
	<-ctx.Done()
}
