package route

import (
	"net/http"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

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

type onDemand struct {
	Medium   string `json:"medium"`             // relative to the API
	Hover    string `json:"hover,omitempty"`    // a video or a Live Photo
	Original string `json:"original,omitempty"` // the biggest of what is seen (a video: its file)
}

func (r *routes) registerRenditions(e *echo.Echo) {
	e.GET("/items/:guid/rendition/:level", r.getRendition)
}

func (r *routes) getRendition(c echo.Context) error {
	item, err := r.db.GetItemByGuid(c.Param("guid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	lib := r.of(item)
	if lib == nil {
		return echo.NewHTTPError(http.StatusNotFound) // a plain folder's: nothing to ask for
	}
	rendition, err := lib.Rendition(item, c.Param("level"), provider.Options{HEVC: c.QueryParam("hevc") != "0"})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	c.Response().Header().Set("Cache-Control", "private, max-age=86400")
	if rendition.Data != nil {
		return c.Blob(http.StatusOK, rendition.Mime, rendition.Data)
	}
	return c.File(rendition.Path)
}

// onDemandOf: the client's on-demand renditions of a library's item (nil for a
// plain folder's) — the levels its library offers, as URLs
func onDemandOf(item *dto.ItemDto, lib Library) *onDemand {
	if lib == nil {
		return nil
	}
	base := "/items/" + item.Guid + "/rendition/"
	// The contract version in the URL: the browser caches these for a day, and what
	// one answers may change with the contract (the Original was the unedited
	// original before 4) — a new contract is a new URL
	v := "?v=" + contractVersion
	od := &onDemand{}
	for _, level := range lib.Levels(item) {
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
