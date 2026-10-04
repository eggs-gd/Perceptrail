// Package library: the libraries of this run — the providers enabled, in the order
// the import's switch asks them (the plain folder last), and which one an item
// belongs to (on demand). The contract they implement is package provider.
package library

import (
	"perceptrail/gontroller/pkg/library/provider"
	"perceptrail/gontroller/pkg/model/dto"
)

var enabled []provider.Provider

// Enable: the libraries of this run, in the order of the chain (set once, at start)
func Enable(libraries ...provider.Provider) { enabled = libraries }

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
