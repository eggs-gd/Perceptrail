package exif_core

import (
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// Largest first, a section where the band changes, ties in the incoming order, no
// value last
func TestOrderByValue(t *testing.T) {
	item := func(guid string, d float64) api.ItemDataProvider { return &dto.ItemDto{Guid: guid, Duration: d} }
	got := OrderByValue([]api.ItemDataProvider{
		item("photo", 0), item("short", 4), item("long", 900), item("mid", 30), item("mid2", 30),
	}, func(it api.ItemDataProvider) float64 { return it.GetDuration() },
		[]Band{{Min: 600, Label: "Over 10 min"}, {Min: 10, Label: "10 s – 10 min"}, {Min: 0, Label: "Under 10 s"}}, "No length")

	want := []string{"long|Over 10 min", "mid|10 s – 10 min", "mid2", "short|Under 10 s", "photo|No length"}
	for i, w := range want {
		s := got[i].Guid
		if got[i].Section != nil {
			s += "|" + got[i].Section.Label
		}
		if s != w {
			t.Errorf("%d: %q, want %q", i, s, w)
		}
	}
}
