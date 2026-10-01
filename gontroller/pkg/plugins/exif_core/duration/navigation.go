package duration

import (
	"context"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
)

// The sheet by length: the longest videos first, sections by length; photos (no
// length) last

const lengthIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="13" r="7.5"/><path d="M12 9v4l2.5 2M10 3.5h4"/></svg>`

var lengths = []exif_core.Band{
	{Min: 600, Label: "Over 10 min"},
	{Min: 60, Label: "1–10 min"},
	{Min: 10, Label: "10 s – 1 min"},
	{Min: 0, Label: "Under 10 s"},
}

func (p *durationPerceptor) View() api.View {
	return api.View{
		Title: "Length",
		Icon:  lengthIcon,
		Help:  "Videos by their length, the longest first; photos follow.",
	}
}

func (p *durationPerceptor) Order(ctx context.Context, _ string, items []api.ItemDataProvider) ([]api.Entry, error) {
	out := exif_core.OrderByValue(items, func(it api.ItemDataProvider) float64 {
		return it.GetDuration()
	}, lengths, "No length")
	return out, ctx.Err()
}
