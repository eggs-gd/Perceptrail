package main

import (
	"context"
	"testing"
	"time"

	"perceptrail/perseptors/exif_geo/places"

	"github.com/eggs-gd/perceplib/api"
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

type item struct {
	guid string
	vals map[string]api.Values
}

func (i *item) GetGuid() string      { return i.guid }
func (i *item) GetDate() time.Time   { return time.Time{} }
func (i *item) GetSize() api.Size    { return api.Size{} }
func (i *item) GetRatio() api.Size   { return api.Size{} }
func (i *item) GetDuration() float64 { return 0 }
func (i *item) StoreValues(s string) (api.Values, bool) {
	v, ok := i.vals[s]
	return v, ok
}
func (i *item) SetStoreValues(s string, v api.Values) { i.vals[s] = v }

// Every city in one piece, though the curve would interleave them; no place last
func TestOrderCitiesInOnePiece(t *testing.T) {
	at := func(guid string, lat, lon float64) api.ItemDataProvider {
		it := &item{guid: guid, vals: map[string]api.Values{}}
		places.Places.Put(it, places.Location{Lat: lat, Lon: lon})
		return it
	}
	items := []api.ItemDataProvider{
		at("kyiv1", 50.45, 30.52), at("minsk", 53.90, 27.56), at("kyiv2", 50.40, 30.60),
		at("tirane1", 41.33, 19.82), at("podgorica", 42.44, 19.26), at("tirane2", 41.30, 19.80),
		&item{guid: "nowhere", vals: map[string]api.Values{}},
	}
	got, err := Perceptor.(*geoPerceptor).Order(context.Background(), "", items)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	last := ""
	var labels []string
	for _, e := range got {
		for _, sec := range e.Sections {
			if seen[sec.Label] {
				t.Errorf("%s comes back: %v", sec.Label, labels)
			}
			seen[sec.Label] = true
			labels = append(labels, sec.Label)
		}
		last = e.Guid
	}
	if last != "nowhere" {
		t.Errorf("no place is not last: %s", last)
	}
	// Every city has its own mark — the region's first one too (a path: region, city)
	for _, city := range []string{"Europe", "Kyiv", "Tirane", "Podgorica", "Minsk", "No place"} {
		if !seen[city] {
			t.Errorf("no section for %s: %v", city, labels)
		}
	}
}
