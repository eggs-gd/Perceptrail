package exif_core

import (
	"cmp"
	"slices"
	"strings"

	"github.com/eggs-gd/perceplib/api"
)

// Band: a section of a sheet ordered by a measure — the items whose value is at
// least Min (bands go from the largest Min down)
type Band struct {
	Min   float64
	Label string
}

// OrderByValue: the sheet by a measure, largest first, sectioned by bands; items
// without a value (≤ 0) go last, under none. Ties keep the incoming order (newest
// first).
func OrderByValue(items []api.ItemDataProvider, value func(api.ItemDataProvider) float64, bands []Band, none string) []api.Entry {
	type measured struct {
		guid string
		v    float64
		at   int
	}
	all := make([]measured, len(items))
	for i, it := range items {
		all[i] = measured{it.GetGuid(), value(it), i}
	}
	slices.SortStableFunc(all, func(a, b measured) int {
		if c := cmp.Compare(b.v, a.v); c != 0 {
			return c
		}
		return cmp.Compare(a.at, b.at)
	})

	out := make([]api.Entry, len(all))
	label := ""
	for i, m := range all {
		out[i].Guid = m.guid
		l := none
		if m.v > 0 {
			for _, b := range bands {
				if m.v >= b.Min {
					l = b.Label
					break
				}
			}
		}
		if l != label && strings.TrimSpace(l) != "" {
			out[i].Section = &api.Section{Level: 0, Label: l}
		}
		label = l
	}
	return out
}
