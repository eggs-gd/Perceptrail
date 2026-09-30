package routes

import (
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
