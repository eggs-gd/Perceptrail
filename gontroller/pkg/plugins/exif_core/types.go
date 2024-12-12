package exif_core

import (
	"perceptrail/api"
	"perceptrail/chain"
	l "perceptrail/logger"
)

type RawItemRW interface {
	api.RawItemR
	api.ItemDataEditor
}

type ExifCorePerceptor interface {
	api.Perceptor
	NewProcessor(chin <-chan RawItemRW, chout chan<- RawItemRW, logger *l.Logger) chain.Processor
}
