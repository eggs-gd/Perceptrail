package size

import (
	"context"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
)

// The sheet by resolution: the biggest photos first, sections by megapixels

const sizeIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3.5" y="3.5" width="17" height="17" rx="2"/><rect x="7.5" y="10.5" width="6" height="6" rx="1"/><path d="M13.5 7.5h3v3M16.5 7.5l-4 4"/></svg>`

var megapixels = []exif_core.Band{
	{Min: 48, Label: "48+ MP"},
	{Min: 20, Label: "20–48 MP"},
	{Min: 10, Label: "10–20 MP"},
	{Min: 4, Label: "4–10 MP"},
	{Min: 1, Label: "1–4 MP"},
	{Min: 0, Label: "Under 1 MP"},
}

func (p *sizePerceptor) View() api.View {
	return api.View{
		Title: "Size",
		Icon:  sizeIcon,
		Help:  "Every photo by its resolution, the biggest first. The panel jumps by megapixels.",
	}
}

func (p *sizePerceptor) Order(ctx context.Context, _ string, items []api.ItemDataProvider) ([]api.Entry, error) {
	out := exif_core.OrderByValue(items, func(it api.ItemDataProvider) float64 {
		s := it.GetSize()
		return float64(s.W) * float64(s.H) / 1e6
	}, megapixels, "Unknown size")
	return out, ctx.Err()
}
