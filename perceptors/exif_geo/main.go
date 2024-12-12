package main

import (
	"perceptrail/api"
	"perceptrail/chain"
	l "perceptrail/logger"
)

type geoPerceptor struct{}

func (p *geoPerceptor) Name() string                       { return "exif_geo" }
func (p *geoPerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *geoPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *geoPerceptor) NewProcessor(chin <-chan api.RawItemR, chout chan<- api.RawItemR, logger *l.Logger) chain.Processor {
	return NewGeotagsProcessor(chin, chout, logger)
}

//export NewPerceptor
func NewPerceptor() api.Perceptor {
	return &geoPerceptor{}
}

func main() {}
