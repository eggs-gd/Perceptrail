package exif

import (
	"slices"
	"testing"
)

type tags map[string]string

func (t tags) GetExif(key string) string { return t[key] }

type asked map[string]bool

func (a asked) GetExif(key string) string { a[key] = true; return "" }

func TestCoordinates(t *testing.T) {
	cases := []struct {
		name     string
		exif     tags
		lat, lon float64
		ok       bool
	}{
		{"signed degrees", tags{"GPSLatitude": "-33.8688", "GPSLongitude": "-151.2093"}, -33.8688, -151.2093, true},
		{"QuickTime, with altitude", tags{"GPSCoordinates": "50.4293 30.5381 175.2"}, 50.4293, 30.5381, true},
		{"QuickTime first", tags{"GPSCoordinates": "1 2", "GPSLatitude": "3", "GPSLongitude": "4"}, 1, 2, true},
		{"one half missing", tags{"GPSLatitude": "50.4"}, 0, 0, false},
		{"not a number", tags{"GPSLatitude": "50 deg 25' 45.48\" N", "GPSLongitude": "30"}, 0, 0, false},
		{"none", tags{}, 0, 0, false},
	}
	for _, c := range cases {
		lat, lon, ok := Coordinates(c.exif)
		if lat != c.lat || lon != c.lon || ok != c.ok {
			t.Errorf("%s: %v %v %v, want %v %v %v", c.name, lat, lon, ok, c.lat, c.lon, c.ok)
		}
	}
}

// Coordinates reads only CoordinateTags: a perceptor declaring them gets them all
func TestCoordinatesReadsDeclaredTags(t *testing.T) {
	a := asked{}
	Coordinates(a)
	for tag := range a {
		if !slices.Contains(CoordinateTags, tag) {
			t.Errorf("Coordinates reads %s, not in CoordinateTags", tag)
		}
	}
}
