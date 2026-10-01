package main

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"perceptrail/perseptors/exif_geo/places"

	"github.com/eggs-gd/perceplib/api"
	"github.com/ringsaturn/tzf"
)

// The sheet by place: the coordinates on a Hilbert curve — one dimension that keeps
// near places near (a plain longitude would put Krakow next to Cape Town). Sections
// are the real places, from the time zone there: its region, then its city
// ("Europe", "Kyiv"), each in one piece. Photos without a place go last.

const geoIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21s-6.5-5.6-6.5-11a6.5 6.5 0 0 1 13 0c0 5.4-6.5 11-6.5 11z"/><circle cx="12" cy="10" r="2.3"/></svg>`

func (p *geoPerceptor) View() api.View {
	return api.View{
		Title: "Place",
		Icon:  geoIcon,
		Help:  "Photos by where they were taken: near places next to each other. The panel jumps by region and city.",
	}
}

func (p *geoPerceptor) Schema() api.Schema { return places.Places.Schema() }

// hilbertOrder: the curve's resolution, 2^16 × 2^16 cells (~300 m at the equator)
const hilbertOrder = 16

func (p *geoPerceptor) Order(ctx context.Context, _ string, items []api.ItemDataProvider) ([]api.Entry, error) {
	type placed struct {
		guid         string
		d            uint64
		region, city string
		at           int
	}
	var located []placed
	var nowhere []string
	for i, it := range items {
		loc, ok := places.Places.Get(it)
		if !ok {
			nowhere = append(nowhere, it.GetGuid())
			continue
		}
		r, c := splitZone(zoneName(loc))
		located = append(located, placed{it.GetGuid(), hilbert(loc), r, c, i})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Every city in one piece, every region too: a region's and a city's place on the
	// curve is its first photo's, so near regions and near cities stay near; within a
	// city, by the curve (near spots together), the same spot newest first. Sorting by
	// the curve alone made a city come back several times (the curve zigzags).
	regionAt, cityAt := map[string]uint64{}, map[[2]string]uint64{}
	for _, p := range located {
		if d, ok := regionAt[p.region]; !ok || p.d < d {
			regionAt[p.region] = p.d
		}
		key := [2]string{p.region, p.city}
		if d, ok := cityAt[key]; !ok || p.d < d {
			cityAt[key] = p.d
		}
	}
	slices.SortStableFunc(located, func(a, b placed) int {
		return cmp.Or(
			cmp.Compare(regionAt[a.region], regionAt[b.region]),
			cmp.Compare(a.region, b.region),
			cmp.Compare(cityAt[[2]string{a.region, a.city}], cityAt[[2]string{b.region, b.city}]),
			cmp.Compare(a.city, b.city),
			cmp.Compare(a.d, b.d),
			cmp.Compare(a.at, b.at),
		)
	})

	out := make([]api.Entry, 0, len(items))
	region, city := "", ""
	for _, p := range located {
		e := api.Entry{Guid: p.guid}
		switch {
		case p.region != region:
			e.Sections = []api.Section{{Level: 0, Label: p.region}}
			if p.city != "" {
				e.Sections = append(e.Sections, api.Section{Level: 1, Label: p.city})
			}
		case p.city != city:
			e.Sections = []api.Section{{Level: 1, Label: p.city}}
		}
		region, city = p.region, p.city
		out = append(out, e)
	}
	for i, guid := range nowhere {
		e := api.Entry{Guid: guid}
		if i == 0 {
			e.Sections = []api.Section{{Level: 0, Label: "No place"}}
		}
		out = append(out, e)
	}
	return out, nil
}

// hilbert: the position of (lat, lon) along a Hilbert curve over the whole globe
// (xy2d — the standard algorithm)
func hilbert(loc places.Location) uint64 {
	const n = 1 << hilbertOrder
	x := uint64((loc.Lon + 180) / 360 * (n - 1))
	y := uint64((loc.Lat + 90) / 180 * (n - 1))
	var d uint64
	for s := uint64(n / 2); s > 0; s /= 2 {
		var rx, ry uint64
		if x&s > 0 {
			rx = 1
		}
		if y&s > 0 {
			ry = 1
		}
		d += s * s * ((3 * rx) ^ ry)
		// Rotate the quadrant so the curve stays continuous
		if ry == 0 {
			if rx == 1 {
				x, y = n-1-x, n-1-y
			}
			x, y = y, x
		}
	}
	return d
}

var (
	finderOnce sync.Once
	finder     tzf.F
)

// zoneName: the IANA zone at the place ("Europe/Kyiv"), "" if unknown
func zoneName(loc places.Location) string {
	finderOnce.Do(func() {
		if f, err := tzf.NewDefaultFinder(); err == nil {
			finder = f
		}
	})
	if finder == nil {
		return ""
	}
	return finder.GetTimezoneName(loc.Lon, loc.Lat)
}

// splitZone: "Europe/Kyiv" → "Europe", "Kyiv"; "America/Argentina/Buenos_Aires" →
// "America", "Buenos Aires"; a zone without a region (the sea: "Etc/GMT+3") →
// "Etc/GMT+3" as both
func splitZone(zone string) (region, city string) {
	if zone == "" {
		return "Unknown zone", ""
	}
	parts := strings.Split(zone, "/")
	if len(parts) < 2 || parts[0] == "Etc" {
		return zone, ""
	}
	return parts[0], strings.ReplaceAll(parts[len(parts)-1], "_", " ")
}
