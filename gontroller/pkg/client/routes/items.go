package routes

import (
	"encoding/json"
	"fmt"
	"gontroller/pkg/model"
	"gontroller/pkg/model/dto"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
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
	Guid     string    `json:"guid"`
	Date     time.Time `json:"date"`
	MimeType string    `json:"mimeType"`
	Width    int16     `json:"width"`
	Height   int16     `json:"height"`
}

func RegisterItemsRoutes(segment string, e *echo.Echo, logger *zap.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
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
				Date:     dbItem.Date,
				MimeType: dbItem.MimeType,
				Width:    16, //int16(dbItem.Ratio.W),
				Height:   9,  //int16(dbItem.Ratio.H),
			}

			itemsChannel <- clientItem
		}
		close(itemsChannel)
	}()

	w.Header().Set("Content-Type", "application/json")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	for clientItem := range itemsChannel {
		//time.Sleep(200 * time.Millisecond)
		if err := encoder.Encode(clientItem); err != nil {
			return err
		}
		flusher.Flush()
	}
	return nil
}
