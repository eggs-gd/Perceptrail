package main

import (
	"context"

	"github.com/eggs-gd/perceplib/api"
)

// The sheet by place. Planned: (lat, lon) on a Hilbert curve — one dimension that
// keeps near places near — with sections by country and city. The perceptor does
// not keep the coordinates yet (no perceptor data storage), so for now every photo
// is "place not known yet", in the incoming order (newest first).

const geoIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21s-6.5-5.6-6.5-11a6.5 6.5 0 0 1 13 0c0 5.4-6.5 11-6.5 11z"/><circle cx="12" cy="10" r="2.3"/></svg>`

func (p *geoPerceptor) View() api.View {
	return api.View{
		Title: "Place",
		Icon:  geoIcon,
		Help:  "Photos by where they were taken (not ready yet: places are not stored).",
	}
}

func (p *geoPerceptor) Order(ctx context.Context, _ string, items []api.ItemDataProvider) ([]api.Entry, error) {
	out := make([]api.Entry, len(items))
	for i, it := range items {
		out[i].Guid = it.GetGuid()
	}
	if len(out) > 0 {
		out[0].Section = &api.Section{Level: 0, Label: "Place not known yet"}
	}
	return out, ctx.Err()
}
