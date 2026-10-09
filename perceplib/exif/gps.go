// Package exif: helpers for the values the core reads with exiftool -n (numbers as
// numbers: signed decimal degrees, seconds, 1–8 orientations), shared by the core's
// perceptors and the plugins. Not the contract (that is api).
package exif

import (
	"strconv"
	"strings"

	"github.com/eggs-gd/perceplib/api"
)

// CoordinateTags: what Coordinates reads — a perceptor that calls it declares them
// (api.ExifTagger)
var CoordinateTags = []string{"GPSCoordinates", "GPSLatitude", "GPSLongitude"}

// Coordinates: latitude and longitude, decimal degrees (south and west negative) —
// QuickTime's GPSCoordinates ("lat lon [alt]"), else GPSLatitude / GPSLongitude
// (exiftool's composite tags: signed by their Ref)
func Coordinates(exif api.ExifProvider) (lat, lon float64, ok bool) {
	if f := strings.Fields(exif.GetExif("GPSCoordinates")); len(f) >= 2 {
		la, err1 := strconv.ParseFloat(f[0], 64)
		lo, err2 := strconv.ParseFloat(f[1], 64)
		if err1 == nil && err2 == nil {
			return la, lo, true
		}
	}
	la, err1 := strconv.ParseFloat(strings.TrimSpace(exif.GetExif("GPSLatitude")), 64)
	lo, err2 := strconv.ParseFloat(strings.TrimSpace(exif.GetExif("GPSLongitude")), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return la, lo, true
}
