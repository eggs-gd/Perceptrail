package main

import (
	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/exif"
)

type geoPerceptor struct{}

func (p *geoPerceptor) Name() string                       { return "exif_geo" }
func (p *geoPerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *geoPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *geoPerceptor) Decorator(logger *l.Logger) chain.Decorator[api.RawItemR, api.RawItemR] {
	return &geotagsExtractor{logger}
}

// ExifTags: the coordinates (exif.Coordinates)
func (p *geoPerceptor) ExifTags() []string { return exif.CoordinateTags }

var Perceptor api.Perceptor = &geoPerceptor{}

func main() {}
