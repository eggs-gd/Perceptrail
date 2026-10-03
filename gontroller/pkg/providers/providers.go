// Package providers: the libraries we read through their own means — Apple Photos
// now, Immich and others later. A library that keeps renditions of its own is asked
// for them; we render and store nothing it keeps.
//
// In the import chain one switch asks the providers in order: a found file goes to
// the grouper of the first that claims it — each grouper a step of its own. The
// plain folder (providers/folder) is a provider too, the last: it claims what nobody
// else did. A provider not enabled is not asked — its files are a plain folder's.
//
// On demand the web service asks the item's provider for a rendition (the viewer's
// medium, a hover, the original) and serves what it gets: a file or bytes.
package providers

import (
	"context"
	"errors"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// Grouper: the logic of a provider's step — found files in, whole assets out
// (chain.Decorator is a step's logic, not the step: no pipes, no goroutine). The
// importer runs it between its pipes (chain.Decorate); on the walk's flush it gives
// what it holds (chain.Flusher: the last group, the files held back). The provider
// keeps the instance — its state is what Regroup works from.
type Grouper = chain.Decorator[dto.ItemEntry, Group]

type Provider interface {
	Name() string

	// Claims: a found file is this library's — its grouper takes it
	Claims(path string) bool
	// Grouper: its files into whole assets; one instance per run (it keeps what
	// Regroup needs)
	Grouper() Grouper
	// Regroup: one asset's group as it is on disk now (processed again on demand);
	// false if the asset is not this provider's or has no file
	Regroup(key string) (Asset, bool)

	// Owns: the item is this library's (on-demand renditions go to it)
	Owns(item *dto.ItemDto) bool
	// Levels: what the client may ask for this item ("medium", "hover", "original")
	Levels(item *dto.ItemDto) []string
	// Rendition: a level of the item — a file, or bytes when the library drew it;
	// ErrNoRendition when there is nothing to serve
	Rendition(item *dto.ItemDto, level string, opt Options) (Rendition, error)

	// Start: its own work in the background (access, assets nothing shows yet);
	// refresh processes one asset's item again when the library made a file local
	Start(ctx context.Context, refresh Refresher)
}

// Options of a rendition request
type Options struct {
	HEVC bool // the browser plays HEVC
}

// Rendition: a file of the library, or bytes it drew (Data with Mime)
type Rendition struct {
	Path string
	Data []byte
	Mime string
}

// Refresher processes one asset's item again now (the importer's Refresh), waiting
// at most wait; false if it did not happen
type Refresher func(key string, wait time.Duration) bool

var ErrNoRendition = errors.New("no rendition")

var enabled []Provider

// Enable: the providers of this run, in the order of the chain (set once, at start)
func Enable(ps ...Provider) { enabled = ps }

// Enabled: the providers in the chain, in order
func Enabled() []Provider { return enabled }

// Of: the provider an item belongs to, nil for a plain folder's
func Of(item *dto.ItemDto) Provider {
	for _, p := range enabled {
		if p.Owns(item) {
			return p
		}
	}
	return nil
}
