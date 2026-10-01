package routes

import (
	"encoding/json"
	"net/http"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

// Navigators: the perceptors that give the gallery its views of the library. The
// client draws a button per navigator, asks one for the order of the sheet and
// lays it out itself.

type clientPerceptor struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Icon     string `json:"icon"` // SVG markup
	Help     string `json:"help"`
	Relative bool   `json:"relative"`
}

type clientEntry struct {
	Guid    string         `json:"guid"`
	Section *clientSection `json:"section,omitempty"`
}

type clientSection struct {
	Level int    `json:"level"`
	Label string `json:"label"`
}

// navigators: the loaded perceptors that navigate, core first (the default view)
var navigators []api.Navigator

// RegisterPerceptorsRoutes: the navigators among the loaded perceptors (the plugin
// manager passes them in: routes cannot import it, app imports client)
func RegisterPerceptorsRoutes(e *echo.Echo, perceptors []api.Perceptor, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	navigators = nil
	for _, p := range perceptors {
		if n, ok := p.(api.Navigator); ok {
			navigators = append(navigators, n)
		}
	}
	e.GET("/perceptors", getPerceptors)
	e.GET("/p/:name/order", getOrder)
}

func getPerceptors(c echo.Context) error {
	out := []clientPerceptor{}
	for _, n := range navigators {
		v := n.View()
		out = append(out, clientPerceptor{Name: n.Name(), Title: v.Title, Icon: v.Icon, Help: v.Help, Relative: v.Relative})
	}
	return c.JSON(http.StatusOK, out)
}

// getOrder streams the sheet in the navigator's order, one entry per line (ndjson),
// over the items the client is shown
func getOrder(c echo.Context) error {
	var nav api.Navigator
	for _, n := range navigators {
		if n.Name() == c.Param("name") {
			nav = n
		}
	}
	if nav == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	items, err := itemsProxy.GetItemsInStates(shownStates...)
	if err != nil {
		return err
	}
	in := make([]api.ItemDataProvider, len(items))
	for i, it := range items {
		in[i] = it
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
		if e.Section != nil {
			out.Section = &clientSection{Level: e.Section.Level, Label: e.Section.Label}
		}
		if err := enc.Encode(out); err != nil {
			return err
		}
	}
	return nil
}

// shownStates: what the client is shown (see shown)
var shownStates = []dto.ItemState{dto.Visible, dto.Ready}
