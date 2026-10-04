// Package provider: the contract of a library we read through its own means — Apple
// Photos now, Immich and others later. A library that keeps renditions of its own is asked
// for them; we render and store nothing it keeps.
//
// In the import chain one switch asks the providers in order: a found file goes to
// the grouper of the first that claims it — each grouper a step of its own. The
// plain folder (library/folder) is a provider too, the last: it claims what nobody
// else did. A provider not enabled is not asked — its files are a plain folder's.
//
// On demand the web service asks the item's provider for a rendition (the viewer's
// medium, a hover, the original) and serves what it gets: a file or bytes.
package provider

import (
	"context"
	"errors"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// Grouper: the logic of a provider's step — the walk's files in, whole assets out
// (chain.Decorator is a step's logic, not the step: no channels, no goroutine). The
// importer runs it between its channels (chain.NewDecorator); when the walk ends (its
// input closes) it gives what it holds (chain.Flusher: the last group).
type Grouper = chain.Decorator[dto.WalkedFile, dto.Asset]

type Provider interface {
	Name() string

	// Claims: a found file is this library's — its grouper takes it
	Claims(path string) bool
	// Grouper: its files into whole assets; one instance per run
	Grouper() Grouper

	// Owns: the item is this library's (on-demand renditions go to it)
	Owns(item *dto.ItemDto) bool
	// Levels: what the client may ask for this item ("medium", "hover", "original")
	Levels(item *dto.ItemDto) []string
	// Rendition: a level of the item — a file, or bytes when the library drew it;
	// ErrNoRendition when there is nothing to serve
	Rendition(item *dto.ItemDto, level string, opt Options) (Rendition, error)

	// Start: its own work in the background (access, assets nothing shows yet)
	Start(ctx context.Context)
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

var ErrNoRendition = errors.New("no rendition")
