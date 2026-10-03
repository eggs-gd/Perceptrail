package exif_core

import (
	"fmt"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins/exif_core/exif_coretest"

	"github.com/eggs-gd/perceplib/api"
)

// Largest first; a section where the coarsest changed level changes (the value,
// then the finer one); ties in the incoming order; no value last under none
func TestOrderByValue(t *testing.T) {
	item := func(guid string, d float64) api.ItemDataProvider {
		return exif_coretest.Stored(&dto.ItemDto{Guid: guid, Duration: d})
	}
	got := OrderByValue([]api.ItemDataProvider{
		item("photo", 0), item("a", 125), item("b", 130), item("c", 61), item("c2", 61),
	}, func(it api.ItemDataProvider) float64 { return it.GetDuration() },
		func(_ api.ItemDataProvider, v float64) []string {
			return []string{fmt.Sprintf("%d min", int(v/60)), fmt.Sprintf("%d s", int(v))}
		}, "No length")

	// A new coarse section starts its finer one too (a path)
	want := []string{"b|0:2 min|1:130 s", "a|1:125 s", "c|0:1 min|1:61 s", "c2", "photo|0:No length"}
	for i, w := range want {
		s := got[i].Guid
		for _, sec := range got[i].Sections {
			s += fmt.Sprintf("|%d:%s", sec.Level, sec.Label)
		}
		if s != w {
			t.Errorf("%d: %q, want %q", i, s, w)
		}
	}
}
