package routes

import (
	"perceptrail/gontroller/pkg/model"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RegisterAssetsRoutes(segment string, e *echo.Echo, logger *zap.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}

	userGroup := e.Group(segment)
	userGroup.GET("/:item", getFile) // all items
}

func getFile(c echo.Context) error {
	guid := c.Param("item")

	item, err := itemsProxy.GetItemByGuid(guid)
	if err != nil {
		return err
	}

	return c.File(item.Path)
}
