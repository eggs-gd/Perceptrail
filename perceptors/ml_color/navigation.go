package main

import (
	"context"

	"github.com/eggs-gd/perceplib/api"
)

// The sheet by colour: a trail from the photo the user is at through photos of
// similar colour (relative). Nothing is analysed yet (the processor is a stub), so
// for now every photo is "not analysed yet", in the incoming order (newest first).

const colorIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><circle cx="9" cy="9.5" r="5"/><circle cx="15" cy="9.5" r="5"/><circle cx="12" cy="14.5" r="5"/></svg>`

func (p *colorPerceptor) View() api.View {
	return api.View{
		Title:    "Colour",
		Icon:     colorIcon,
		Help:     "A trail through photos of similar colour (not ready yet: nothing is analysed).",
		Relative: true,
	}
}

func (p *colorPerceptor) Order(ctx context.Context, _ string, items []api.ItemDataProvider) ([]api.Entry, error) {
	out := make([]api.Entry, len(items))
	for i, it := range items {
		out[i].Guid = it.GetGuid()
	}
	if len(out) > 0 {
		out[0].Section = &api.Section{Level: 0, Label: "Not analysed yet"}
	}
	return out, ctx.Err()
}

// Schema: nothing kept yet (the processor is a stub)
func (p *colorPerceptor) Schema() api.Schema { return api.Schema{} }
