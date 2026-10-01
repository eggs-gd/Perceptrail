package duration

import (
	"context"
	"fmt"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
)

// The sheet by length: the longest videos first; sections are the real lengths —
// whole minutes, whole seconds under a minute (tenths under a second); photos (no
// length) last

const lengthIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="13" r="7.5"/><path d="M12 9v4l2.5 2M10 3.5h4"/></svg>`

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
	}, func(_ api.ItemDataProvider, seconds float64) []string {
		return []string{length(seconds)}
	}, "No length")
	return out, ctx.Err()
}

// length: whole minutes, whole seconds under a minute, tenths under a second
// ("12 min", "45 s", "0.4 s")
func length(seconds float64) string {
	switch {
	case seconds >= 60:
		return fmt.Sprintf("%d min", int(seconds/60))
	case seconds >= 1:
		return fmt.Sprintf("%d s", int(seconds))
	}
	return fmt.Sprintf("%.1f s", seconds)
}
