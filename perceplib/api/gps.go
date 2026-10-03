package api

import (
	"regexp"
	"strconv"
)

// Coordinates as exiftool prints them: 50 deg 27' 12.34" N
var dmsRe = regexp.MustCompile(`(\d+(?:\.\d+)?) deg (\d+(?:\.\d+)?)' (\d+(?:\.\d+)?)"(?: ([NSEW]))?`)

// CoordinateTags: what Coordinates reads — a perceptor that calls it declares them
// (ExifTagger)
var CoordinateTags = []string{"GPSCoordinates", "GPSLatitude", "GPSLongitude", "GPSLatitudeRef", "GPSLongitudeRef"}

// Coordinates: latitude and longitude (degrees, south and west negative) from
// GPSLatitude/GPSLongitude or the QuickTime GPSCoordinates
func Coordinates(exif ExifProvider) (lat, lon float64, ok bool) {
	if c := exif.GetExif("GPSCoordinates"); c != "" {
		m := dmsRe.FindAllStringSubmatch(c, 2)
		if len(m) == 2 {
			return dms(m[0], exif.GetExif("GPSLatitudeRef")), dms(m[1], exif.GetExif("GPSLongitudeRef")), true
		}
	}
	la := dmsRe.FindStringSubmatch(exif.GetExif("GPSLatitude"))
	lo := dmsRe.FindStringSubmatch(exif.GetExif("GPSLongitude"))
	if la == nil || lo == nil {
		return 0, 0, false
	}
	return dms(la, exif.GetExif("GPSLatitudeRef")), dms(lo, exif.GetExif("GPSLongitudeRef")), true
}

// dms converts a dmsRe match to degrees; the direction comes from the value or the Ref tag
func dms(m []string, ref string) float64 {
	d, _ := strconv.ParseFloat(m[1], 64)
	min, _ := strconv.ParseFloat(m[2], 64)
	sec, _ := strconv.ParseFloat(m[3], 64)
	v := d + min/60 + sec/3600
	dir := m[4]
	if dir == "" && ref != "" {
		dir = ref[:1] // "North", "South", "East", "West"
	}
	if dir == "S" || dir == "W" {
		v = -v
	}
	return v
}
