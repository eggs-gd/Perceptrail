package main

import (
	"testing"

	"perceptrail/perseptors/exif_geo/places"
)

// The curve keeps near places near: two spots in Kyiv are much closer along it
// than Kyiv and Cape Town, though Cape Town has nearly the same longitude as Kyiv
func TestHilbertLocality(t *testing.T) {
	kyiv := hilbert(places.Location{Lat: 50.45, Lon: 30.52})
	kyiv2 := hilbert(places.Location{Lat: 50.40, Lon: 30.60})
	cape := hilbert(places.Location{Lat: -33.92, Lon: 18.42})
	dist := func(a, b uint64) uint64 {
		if a > b {
			return a - b
		}
		return b - a
	}
	if dist(kyiv, kyiv2) >= dist(kyiv, cape) {
		t.Errorf("kyiv-kyiv2 %d, kyiv-cape %d", dist(kyiv, kyiv2), dist(kyiv, cape))
	}
	// Every cell has its own position (a bijection on a small grid is enough to
	// catch a wrong rotation)
	seen := map[uint64]bool{}
	for lat := -80.0; lat <= 80; lat += 20 {
		for lon := -170.0; lon <= 170; lon += 20 {
			d := hilbert(places.Location{Lat: lat, Lon: lon})
			if seen[d] {
				t.Fatalf("two places at %d", d)
			}
			seen[d] = true
		}
	}
}

func TestSplitZone(t *testing.T) {
	for zone, want := range map[string][2]string{
		"Europe/Kyiv":                    {"Europe", "Kyiv"},
		"America/Argentina/Buenos_Aires": {"America", "Buenos Aires"},
		"Etc/GMT+3":                      {"Etc/GMT+3", ""},
		"":                               {"Unknown zone", ""},
	} {
		if r, c := splitZone(zone); r != want[0] || c != want[1] {
			t.Errorf("%q: %q %q, want %q", zone, r, c, want)
		}
	}
}
