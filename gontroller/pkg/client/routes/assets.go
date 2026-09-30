package routes

import (
	"perceptrail/gontroller/pkg/model"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

func RegisterAssetsRoutes(segment string, e *echo.Echo, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}

	userGroup := e.Group(segment)
	userGroup.GET("/:item", getFile) // the best that can be shown
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
