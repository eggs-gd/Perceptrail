package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"time"

	l "github.com/dukobpa3/perceplib/logger"

	"github.com/labstack/echo/v4"
)

var itemsProxy model.ItemsApi

type clientItem struct {
	Id       uint      `json:"id"`
	Guid     string    `json:"guid"`
	Date     time.Time `json:"date"`
	MimeType string    `json:"mimeType"`
	Width    int16     `json:"width"`
	Height   int16     `json:"height"`
}

func RegisterItemsRoutes(segment string, e *echo.Echo, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}

	userGroup := e.Group(segment)
	userGroup.GET("/", getItems) // all items
	userGroup.GET("", getItems)  // all items
}

func getItems(c echo.Context) error {
	return streamClientItems(c.Response().Writer)
}

func toClientItem(dbItem *dto.ItemDto) clientItem {
	item := clientItem{
		Id:       dbItem.ID,
		Guid:     dbItem.Guid,
		Date:     dbItem.Date,
		MimeType: dbItem.MimeType,
	}

	if dbItem.Ratio.H == 0 || dbItem.Ratio.W == 0 {
		item.Height = 1
		item.Width = 1
	} else {
		item.Width = int16(dbItem.Ratio.W)
		item.Height = int16(dbItem.Ratio.H)
	}
	return item
}

func streamClientItems(w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	flusher.Flush()

	encoder := json.NewEncoder(w)

	return itemsProxy.StreamAllItems(func(dbItem *dto.ItemDto) error {
		if err := encoder.Encode(toClientItem(dbItem)); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
}
