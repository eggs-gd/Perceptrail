package route

import (
	"encoding/json"
	"net/http"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

// Perceptors are the gallery's ways through the library: the client draws a button
// per perceptor, asks one for the order of the sheet and lays it out itself.

type clientPerceptor struct {
	Slug     string `json:"slug"` // the view's name in URLs; the plugin's name stays inside
	Title    string `json:"title"`
	Icon     string `json:"icon"` // SVG markup
	Help     string `json:"help"`
	Relative bool   `json:"relative"`
}

type clientEntry struct {
	Guid     string          `json:"guid"`
	Sections []clientSection `json:"sections,omitempty"` // started here, coarsest first
}

type clientSection struct {
	Level int    `json:"level"`
	Label string `json:"label"`
}

// ValuesLoader: a perceptor's stored values for these items (store name, by guid)
type ValuesLoader func(perceptor string, guids []string) (string, map[string]api.Values, error)

// registerPerceptors: list is what the client is given, core first (the first is
// the default view)
func (r *routes) registerPerceptors(e *echo.Echo, list []api.Perceptor) {
	// A view is reached by its slug: an empty or a taken one is not reachable
	r.perceptors = nil
	taken := map[string]string{}
	for _, p := range list {
		slug := p.View().Slug
		if other, dup := taken[slug]; slug == "" || dup {
			r.logger.Error("Perceptor view not given to the client: no slug, or taken",
				l.String("perceptor", p.Name()), l.String("slug", slug), l.String("taken by", other))
			continue
		}
		taken[slug] = p.Name()
		r.perceptors = append(r.perceptors, p)
	}
	e.GET("/perceptors", r.getPerceptors)
	e.GET("/p/:view/order", r.getOrder)
	e.GET("/items/:guid/info", r.getInfo)
}

func (r *routes) getPerceptors(c echo.Context) error {
	out := []clientPerceptor{}
	for _, n := range r.perceptors {
		v := n.View()
		out = append(out, clientPerceptor{Slug: v.Slug, Title: v.Title, Icon: v.Icon, Help: v.Help, Relative: v.Relative})
	}
	return c.JSON(http.StatusOK, out)
}

// r.getOrder streams the sheet in the perceptor's order, one entry per line (ndjson),
// over the items the client is shown
func (r *routes) getOrder(c echo.Context) error {
	var nav api.Perceptor
	for _, n := range r.perceptors {
		if n.View().Slug == c.Param("view") {
			nav = n
		}
	}
	if nav == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	items, err := r.db.GetItemsInStates(shownStates...)
	if err != nil {
		return err
	}
	in := make([]api.ItemDataProvider, len(items))
	byGuid := make(map[string]*perceived, len(items))
	guids := make([]string, len(items))
	for i, it := range items {
		p := &perceived{ItemDto: it}
		in[i], byGuid[it.Guid] = p, p
		guids[i] = it.Guid
	}
	// The perceptor's own values ride on the items it orders
	if r.values != nil {
		store, values, err := r.values(nav.Name(), guids)
		if err != nil {
			return err
		}
		for guid, v := range values {
			if p, ok := byGuid[guid]; ok {
				p.SetStoreValues(store, v)
			}
		}
	}
	entries, err := nav.Order(c.Request().Context(), c.QueryParam("anchor"), in)
	if err != nil {
		return err
	}

	w := c.Response()
	w.Header().Set(echo.HeaderContentType, "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	for _, e := range entries {
		out := clientEntry{Guid: e.Guid}
		for _, s := range e.Sections {
			out.Sections = append(out.Sections, clientSection{Level: s.Level, Label: s.Label})
		}
		if err := enc.Encode(out); err != nil {
			return err
		}
	}
	return nil
}

type clientInfo struct {
	Slug  string       `json:"slug"`
	Title string       `json:"title"`
	Icon  string       `json:"icon"`
	Facts []clientFact `json:"facts"`
}

type clientFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// r.getInfo: what every perceptor knows about one item (the viewer's info panel), each
// with its own values loaded; a perceptor that says nothing is left out
func (r *routes) getInfo(c echo.Context) error {
	item, err := r.db.GetItemByGuid(c.Param("guid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	out := []clientInfo{}
	pi := &perceived{ItemDto: item}
	for _, p := range r.perceptors {
		if r.values != nil {
			store, values, err := r.values(p.Name(), []string{item.Guid})
			if err != nil {
				return err
			}
			if v, ok := values[item.Guid]; ok {
				pi.SetStoreValues(store, v)
			}
		}
		facts := p.Info(pi)
		if len(facts) == 0 {
			continue
		}
		v := p.View()
		info := clientInfo{Slug: v.Slug, Title: v.Title, Icon: v.Icon}
		for _, f := range facts {
			info.Facts = append(info.Facts, clientFact{Label: f.Label, Value: f.Value})
		}
		out = append(out, info)
	}
	return c.JSON(http.StatusOK, out)
}

// shownStates: what the client is shown (see shown)
var shownStates = []dto.ItemState{dto.Visible, dto.Ready}

// perceived: a stored item as a perceptor reads it (api.ItemDataProvider) — the
// item, and the r.perceptors' values loaded for it (each perceptor's storage keeps
// them, not the items table)
type perceived struct {
	*dto.ItemDto
	values map[string]api.Values
}

func (p *perceived) GetGuid() string      { return p.Guid }
func (p *perceived) GetSize() api.Size    { return p.Size }
func (p *perceived) GetRatio() api.Size   { return p.Ratio }
func (p *perceived) GetDuration() float64 { return p.Duration }

func (p *perceived) StoreValues(store string) (api.Values, bool) {
	v, ok := p.values[store]
	return v, ok
}

func (p *perceived) SetStoreValues(store string, v api.Values) {
	if p.values == nil {
		p.values = map[string]api.Values{}
	}
	p.values[store] = v
}
