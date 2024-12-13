package exif_core

import (
	"github.com/dukobpa3/perceplib/api"
	"github.com/dukobpa3/perceplib/chain"
	l "github.com/dukobpa3/perceplib/logger"
)

type RawItemRW interface {
	api.RawItemR
	api.ItemDataEditor
}

type ExifCorePerceptor interface {
	api.Perceptor
	NewProcessor(chin <-chan RawItemRW, chout chan<- RawItemRW, logger *l.Logger) chain.Processor
}
