package routes

import (
	"encoding/json"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
	"gontroller/pkg/model/dto"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// https://echo.labstack.com/docs/quick-start
// https://habr.com/en/companies/ozonbank/articles/817381/
/*
	e.POST("/users", saveUser)
	e.GET("/users/:id", getUser)
	e.PUT("/users/:id", updateUser)
	e.DELETE("/users/:id", deleteUser)

	// e.GET("/users/:id", getUser)
	func getUser(c echo.Context) error {
	  	// User ID from path `users/:id`
	  	id := c.Param("id")
		return c.String(http.StatusOK, id)
	}

	/show?team=x-men&member=wolverine
	//e.GET("/show", show)
	func show(c echo.Context) error {
		// Get team and member from the query string
		team := c.QueryParam("team")
		member := c.QueryParam("member")
		return c.String(http.StatusOK, "team:" + team + ", member:" + member)
	}
*/

var itemsProxy model.ItemsApi

type clientItem struct {
	Guid     string
	MimeType string
	Date     time.Time
	Path     string
	Size     t.Size
	Ratio    t.Size
}

func RegisterItemsRoutes(segment string, e *echo.Echo) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy()
	}

	userGroup := e.Group(segment)
	userGroup.GET("/", getItems) // all items
	userGroup.GET("", getItems)  // all items
}

func getItems(c echo.Context) error {
	items, err := itemsProxy.GetAllItems()
	if err != nil {
		return err
	}

	return streamClientItems(items, c.Response().Writer)
}

func streamClientItems(dbItems []*dto.ItemDto, w http.ResponseWriter) error {
	itemsChannel := make(chan clientItem)

	go func() {
		for _, dbItem := range dbItems {
			clientItem := clientItem{
				Guid:     dbItem.Guid,
				MimeType: dbItem.MimeType,
				Date:     dbItem.Date,
				Path:     dbItem.Path,
				Size:     dbItem.Size,
				Ratio:    dbItem.Ratio,
			}
			itemsChannel <- clientItem
		}
		close(itemsChannel)
	}()

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	w.Write([]byte("["))
	first := true

	for clientItem := range itemsChannel {
		if !first {
			w.Write([]byte(","))
		}
		first = false
		if err := encoder.Encode(clientItem); err != nil {
			return err
		}
	}

	w.Write([]byte("]"))
	return nil
}
