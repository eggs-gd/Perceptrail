package routes

import (
	"perceptrail/gontroller/pkg/model"

	l "github.com/dukobpa3/perceplib/logger"

	"github.com/labstack/echo/v4"
)

func RegisterAssetsRoutes(segment string, e *echo.Echo, logger *l.Logger) {
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
