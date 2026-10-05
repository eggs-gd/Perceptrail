package api

import (
	"context"

	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
)

// DataProviderType defines the source of data for the perceptor
type DataProviderType int

const (
	ExifDataProvider DataProviderType = iota
	RawDataProvider
	MetadataProvider
	// Will be extended later with other providers...
)

// ProcessingMode defines how perceptor handles items
type ProcessingMode int

const (
	SingleItem ProcessingMode = iota
	ItemGroup
	// Potentially more modes in future...
)

// Perceptor interface defines the core methods for plugins. Every perceptor
// navigates (see navigation.go): its view and its order of the sheet.
type Perceptor interface {
	Name() string // Unique plugin name
	DataProvider() DataProviderType
	ProcessingMode() ProcessingMode

	View() View
	// Schema: what the perceptor keeps per item (a Store[T]'s Schema); the zero
	// Schema keeps nothing
	Schema() Schema
	// Info: what the perceptor knows about one item, for the viewer's info panel
	// (its values loaded, as for Order); none: the perceptor says nothing about it
	Info(item ItemDataProvider) []Fact
	// Order: every item of items (they come newest first), in sheet order. anchor:
	// the item the user is at ("" for none) — a relative perceptor starts from it.
	Order(ctx context.Context, anchor string, items []ItemDataProvider) ([]Entry, error)
}

// ExifTagger: the tags a perceptor reads (exiftool's names). The core reads only
// the declared tags of the enabled perceptors, from every file of the asset (the
// source's metadata first, then the sidecars, the main file, the derivatives):
// GetExif returns "" for a tag nobody declared. Values are exiftool's -n form —
// numbers as numbers (signed decimal degrees, seconds, Orientation 1–8); dates as
// "2006:01:02 15:04:05". Plugins never run exiftool (helpers: package exif).
type ExifTagger interface {
	ExifTags() []string
}

// ExifPerceptor is a specialized interface for EXIF-based processors
type ExifPerceptor interface {
	Perceptor
	ExifTagger
	// Decorator: the perceptor's logic over one item (it only reads it; its values
	// go to its Store); the core runs it as a step. nil: nothing to do on import.
	Decorator(logger *l.Logger) chain.Decorator[RawItemR, RawItemR]
}
