package exif_core

import (
	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type RawItemRW interface {
	api.RawItemR
	api.ItemDataEditor
}

type ExifCorePerceptor interface {
	api.Perceptor
	NewProcessor(chin <-chan RawItemRW, chout chan<- RawItemRW, logger *l.Logger) chain.Processor
}
