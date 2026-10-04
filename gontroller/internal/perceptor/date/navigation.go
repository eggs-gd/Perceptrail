package date

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/eggs-gd/perceplib/api"
)

// The date is the first perceptor and the default sheet: newest first, sections by
// year and month in the zone of the shot (a photo taken on 31 December in Kyiv is
// in December, wherever the server is). Items without a date go last.

const dateIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><rect x="3.5" y="5" width="17" height="15" rx="2"/><path d="M3.5 9.5h17M8 3v4M16 3v4"/><path d="M7.5 13h2M11 13h2M14.5 13h2M7.5 16.5h2M11 16.5h2"/></svg>`

func (p *datePerceptor) View() api.View {
	return api.View{
		Slug:  "date",
		Title: "Date",
		Icon:  dateIcon,
		Help:  "Every photo by the date it was taken, newest first. The panel on the right jumps to a year or a month.",
	}
}

func (p *datePerceptor) Order(ctx context.Context, _ string, items []api.ItemDataProvider) ([]api.Entry, error) {
	type dated struct {
		guid string
		at   time.Time // in the zone of the shot
	}
	all := make([]dated, 0, len(items))
	for _, it := range items {
		all = append(all, dated{it.GetGuid(), it.GetDate()})
	}
	// Newest first; no date last; ties by guid, so the order is stable
	slices.SortStableFunc(all, func(a, b dated) int {
		switch {
		case a.at.IsZero() != b.at.IsZero():
			if a.at.IsZero() {
				return 1
			}
			return -1
		case !a.at.Equal(b.at):
			return b.at.Compare(a.at)
		}
		return strings.Compare(a.guid, b.guid)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	out := make([]api.Entry, len(all))
	year, month := -1, time.Month(0)
	undated := false
	for i, d := range all {
		out[i].Guid = d.guid
		switch {
		case d.at.IsZero():
			if !undated {
				undated = true
				out[i].Sections = []api.Section{{Level: 0, Label: "No date"}}
			}
		case d.at.Year() != year:
			year, month = d.at.Year(), d.at.Month()
			out[i].Sections = []api.Section{{Level: 0, Label: fmt.Sprint(year)}, {Level: 1, Label: month.String()}}
		case d.at.Month() != month:
			month = d.at.Month()
			out[i].Sections = []api.Section{{Level: 1, Label: month.String()}}
		}
	}
	return out, nil
}

// Schema: nothing of its own to keep — the core's item has it
func (p *datePerceptor) Schema() api.Schema { return api.Schema{} }

// Info: when the photo was taken, in the zone of the shot
func (p *datePerceptor) Info(item api.ItemDataProvider) []api.Fact {
	at := item.GetDate()
	if at.IsZero() {
		return []api.Fact{{Label: "Taken", Value: "unknown"}}
	}
	return []api.Fact{{Label: "Taken", Value: at.Format("2 Jan 2006, 15:04 -07:00")}}
}
