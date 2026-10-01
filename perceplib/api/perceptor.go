package api

import (
	"context"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
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
	// Order: every item of items (they come newest first), in sheet order. anchor:
	// the item the user is at ("" for none) — a relative perceptor starts from it.
	Order(ctx context.Context, anchor string, items []ItemDataProvider) ([]Entry, error)
}

// ExifPerceptor is a specialized interface for EXIF-based processors
type ExifPerceptor interface {
	Perceptor
	NewProcessor(chin <-chan RawItemR, chout chan<- RawItemR, logger *l.Logger) chain.Processor
}
