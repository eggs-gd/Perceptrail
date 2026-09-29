package main

import (
	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

type geotagsExtractor struct {
	logger *l.Logger
}

func (cd *geotagsExtractor) Decorate(in api.RawItemR) (api.RawItemR, error) {
	// Extract geo data from EXIF
	/*
		lat, _ := strconv.ParseFloat(string(in.Exif[0]["GPSLatitude"]), 64)
		lon, _ := strconv.ParseFloat(string(in.Exif[0]["GPSLongitude"]), 64)

		// Update if changed
		if in.Item.GeoData.Latitude != lat || in.Item.GeoData.Longitude != lon {
			in.Item.GeoData = dto.GeoData{
				Latitude:  lat,
				Longitude: lon,
			}
			in.Item.State = dto.Dirty
		}
	*/

	return in, nil
}

func (cd *geotagsExtractor) Stop() {}

func NewGeotagsProcessor(chin <-chan api.RawItemR, chout chan<- api.RawItemR, logger *l.Logger) chain.Processor {
	processor := &geotagsExtractor{logger}

	return chain.NewDecorator(chin, chout, processor)
}
