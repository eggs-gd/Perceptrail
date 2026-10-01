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

// Perceptors are the gallery's ways through the library: the client draws a button
// per perceptor, asks one for the order of the sheet and lays it out itself.

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

// perceptors: the ones the client is given, core first (the first is the default
// view)
var perceptors []api.Perceptor

// RegisterPerceptorsRoutes: list is what the client is given (the plugin manager
// passes it in — config `client`; routes cannot import it, app imports client)
func RegisterPerceptorsRoutes(e *echo.Echo, list []api.Perceptor, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	perceptors = list
	e.GET("/perceptors", getPerceptors)
	e.GET("/p/:name/order", getOrder)
}

func getPerceptors(c echo.Context) error {
	out := []clientPerceptor{}
	for _, n := range perceptors {
		v := n.View()
		out = append(out, clientPerceptor{Name: n.Name(), Title: v.Title, Icon: v.Icon, Help: v.Help, Relative: v.Relative})
	}
	return c.JSON(http.StatusOK, out)
}

// getOrder streams the sheet in the perceptor's order, one entry per line (ndjson),
// over the items the client is shown
func getOrder(c echo.Context) error {
	var nav api.Perceptor
	for _, n := range perceptors {
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
