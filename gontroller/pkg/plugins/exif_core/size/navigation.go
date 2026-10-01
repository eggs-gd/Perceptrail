package size

import (
	"context"
	"fmt"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
)

// The sheet by resolution: the biggest photos first; sections are the real values —
// megapixels, then the exact size

const sizeIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3.5" y="3.5" width="17" height="17" rx="2"/><rect x="7.5" y="10.5" width="6" height="6" rx="1"/><path d="M13.5 7.5h3v3M16.5 7.5l-4 4"/></svg>`

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
	}, func(it api.ItemDataProvider, mp float64) []string {
		s := it.GetSize()
		long, short := max(s.W, s.H), min(s.W, s.H) // a portrait and a landscape are the same size
		exact := fmt.Sprintf("%d×%d", long, short)
		if mp < 0.1 {
			return []string{exact} // icons, thumbnails: megapixels would read "0.0"
		}
		return []string{megapixels(mp), exact}
	}, "Unknown size")
	return out, ctx.Err()
}

// megapixels: whole ones, tenths below one ("12 MP", "0.3 MP")
func megapixels(mp float64) string {
	if mp >= 1 {
		return fmt.Sprintf("%d MP", int(mp))
	}
	return fmt.Sprintf("%.1f MP", mp)
}
