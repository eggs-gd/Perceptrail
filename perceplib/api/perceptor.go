package api

import (
	"perceptrail/chain"
	l "perceptrail/logger"
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

// Perceptor interface defines the core methods for plugins
type Perceptor interface {
	Name() string // Unique plugin name
	DataProvider() DataProviderType
	ProcessingMode() ProcessingMode
}

// ExifPerceptor is a specialized interface for EXIF-based processors
type ExifPerceptor interface {
	Perceptor
	NewProcessor(chin <-chan RawItemR, chout chan<- RawItemR, logger *l.Logger) chain.Processor
}
