package route

import (
	"net/http"

	"perceptrail/gontroller/pkg/library"
	"perceptrail/gontroller/pkg/library/provider"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

// On demand: an item of a provider's library (Apple Photos…) may be asked for a
// rendition its library keeps only remotely — the provider gets it (see its
// package) and this serves what it gives: a file, or bytes it drew.
//
//	GET /items/:guid/rendition/:level   medium (the viewer), hover (a tile's hover),
//	                                    original (the biggest of what is seen);
//	                                    ?hevc=0: the browser plays no HEVC
//
// Nothing to serve (not a provider's item, nothing there, no access): 404, the
// client keeps what it shows.

func RegisterRenditionRoutes(e *echo.Echo, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	e.GET("/items/:guid/rendition/:level", getRendition)
}

func getRendition(c echo.Context) error {
	item, err := itemsProxy.GetItemByGuid(c.Param("guid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	p := library.Of(item)
	if p == nil {
		return echo.NewHTTPError(http.StatusNotFound) // a plain folder's: nothing to ask for
	}
	r, err := p.Rendition(item, c.Param("level"), provider.Options{HEVC: c.QueryParam("hevc") != "0"})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	c.Response().Header().Set("Cache-Control", "private, max-age=86400")
	if r.Data != nil {
		return c.Blob(http.StatusOK, r.Mime, r.Data)
	}
	return c.File(r.Path)
}

// onDemandOf: the client's on-demand renditions of a provider's item (nil for a
// plain folder's) — the levels its provider offers, as URLs
func onDemandOf(item *dto.ItemDto) *onDemand {
	p := library.Of(item)
	if p == nil {
		return nil
	}
	base := "/items/" + item.Guid + "/rendition/"
	// The contract version in the URL: the browser caches these for a day, and what
	// one answers may change with the contract (the Original was the unedited
	// original before 4) — a new contract is a new URL
	v := "?v=" + contractVersion
	od := &onDemand{}
	for _, level := range p.Levels(item) {
		switch level {
		case "medium":
			od.Medium = base + level + v
		case "hover":
			od.Hover = base + level + v
		case "original":
			od.Original = base + level + v
		}
	}
	return od
}

type onDemand struct {
	Medium   string `json:"medium"`             // relative to the API
	Hover    string `json:"hover,omitempty"`    // a video or a Live Photo
	Original string `json:"original,omitempty"` // the biggest of what is seen (a video: its file)
}
