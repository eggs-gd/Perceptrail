package main

import (
	"math"

	"perceptrail/perseptors/exif_geo/places"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/exif"

	l "github.com/eggs-gd/perceplib/logger"
)

// geotagsExtractor keeps where the photo was taken: the coordinates from EXIF (or
// the Photos DB's record, read first). No coordinates: nothing is put, the core
// records "processed, nothing found".
type geotagsExtractor struct {
	logger *l.Logger
}

func (cd *geotagsExtractor) Decorate(in api.RawItemR) (api.RawItemR, error) {
	lat, lon, ok := exif.Coordinates(in)
	// 0,0 is a camera that had no fix, not the Gulf of Guinea
	if ok && math.Abs(lat) <= 90 && math.Abs(lon) <= 180 && (lat != 0 || lon != 0) {
		places.Places.Put(in, places.Location{Lat: lat, Lon: lon})
	}
	return in, nil
}
