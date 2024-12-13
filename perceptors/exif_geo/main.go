package main

import (
	"github.com/dukobpa3/perceplib/api"
	"github.com/dukobpa3/perceplib/chain"
	l "github.com/dukobpa3/perceplib/logger"
)

type geoPerceptor struct{}

func (p *geoPerceptor) Name() string                       { return "exif_geo" }
func (p *geoPerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *geoPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *geoPerceptor) NewProcessor(chin <-chan api.RawItemR, chout chan<- api.RawItemR, logger *l.Logger) chain.Processor {
	return NewGeotagsProcessor(chin, chout, logger)
}

var Perceptor api.Perceptor = &geoPerceptor{}

func main() {}
