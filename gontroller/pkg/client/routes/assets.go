package routes

import (
	"gontroller/pkg/model"

	"github.com/labstack/echo/v4"
)

func RegisterAssetsRoutes(segment string, e *echo.Echo) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy()
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
