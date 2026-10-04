package builtin

import (
	"cmp"
	"slices"

	"github.com/eggs-gd/perceplib/api"
)

// OrderByValue: the sheet by a measure, largest first. Sections come from the real
// values, like years and months for the date: labels(item, value) gives them per
// level, coarsest first ("12 MP", "4032×3024"); a section starts where a level's
// label changes (and every finer one with it). Items without a value (≤ 0) go last, under none — the only fixed
// label. Ties keep the incoming order (newest first).
func OrderByValue(items []api.ItemDataProvider, value func(api.ItemDataProvider) float64,
	labels func(api.ItemDataProvider, float64) []string, none string) []api.Entry {
	type measured struct {
		it api.ItemDataProvider
		v  float64
		at int
	}
	all := make([]measured, len(items))
	for i, it := range items {
		all[i] = measured{it, value(it), i}
	}
	slices.SortStableFunc(all, func(a, b measured) int {
		if c := cmp.Compare(b.v, a.v); c != 0 {
			return c
		}
		return cmp.Compare(a.at, b.at)
	})

	out := make([]api.Entry, len(all))
	var prev []string
	for i, m := range all {
		out[i].Guid = m.it.GetGuid()
		cur := []string{none}
		if m.v > 0 {
			cur = labels(m.it, m.v)
		}
		// From the coarsest level that changed down, every level starts a section
		for level, label := range cur {
			if out[i].Sections != nil || level >= len(prev) || prev[level] != label {
				out[i].Sections = append(out[i].Sections, api.Section{Level: level, Label: label})
			}
		}
		prev = cur
	}
	return out
}
