package routes

import (
	"fmt"
	"net/http"
	"strconv"

	"perceptrail/gontroller/pkg/model"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

var filesProxy model.FilesApi

func RegisterAssetsRoutes(segment string, e *echo.Echo, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	if filesProxy == nil {
		filesProxy = model.NewProxy(logger)
	}

	userGroup := e.Group(segment)
	userGroup.GET("/:item", getFile)            // the default preview
	userGroup.GET("/:item/:file", getAssetFile) // any file of the asset (the asset contract)
	e.GET("/items/:guid/files", getItemFiles)
}

// itemFile: one file of the item's group, for the info panel (sidecars too)
type itemFile struct {
	Name string `json:"name"`
	Role string `json:"role"` // original, edit, still, motion, frames, meta; "" not media
	Mime string `json:"mime"`
	Size int64  `json:"size"`
	W    int    `json:"w,omitempty"`
	H    int    `json:"h,omitempty"`
	URL  string `json:"url"` // relative to the API: a download
}

// getItemFiles: every file of the item's group — the original, its edits,
// derivatives, sidecars, frames — each with a URL to download it
func getItemFiles(c echo.Context) error {
	guid := c.Param("guid")
	files, err := filesProxy.GetLinkedFiles(guid)
	if err != nil {
		return err
	}
	out := []itemFile{}
	for _, f := range files {
		out = append(out, itemFile{Name: f.Name, Role: f.Role, Mime: f.MimeType, Size: f.Size,
			W: f.Width, H: f.Height, URL: fmt.Sprintf("/assets/%s/%d", guid, f.ID)})
	}
	return c.JSON(http.StatusOK, out)
}

func getFile(c echo.Context) error {
	guid := c.Param("item")

	item, err := itemsProxy.GetItemByGuid(guid)
	if err != nil {
		return err
	}

	// The cheap preview until our own previews exist; items from before the cheap
	// stage: the original
	if item.PreviewPath != "" {
		return c.File(item.PreviewPath)
	}
	return c.File(item.Path)
}

// getAssetFile serves a file of the asset by its id (or the extracted embedded
// preview) — only a file linked to this asset, never an arbitrary path
func getAssetFile(c echo.Context) error {
	guid, name := c.Param("item"), c.Param("file")

	if name == embeddedName {
		item, err := itemsProxy.GetItemByGuid(guid)
		if err != nil || item.PreviewPath == "" {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return c.File(item.PreviewPath)
	}

	id, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	f, err := filesProxy.GetFileByID(uint(id))
	if err != nil || f.LinkedTo != guid {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return c.File(f.Path)
}
