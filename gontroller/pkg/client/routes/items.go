package routes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"slices"
	"time"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

var itemsProxy model.ItemsApi

type clientItem struct {
	Id       uint      `json:"id"`
	Guid     string    `json:"guid"`
	Date     time.Time `json:"date"`
	MimeType string    `json:"mimeType"`
	// What /assets/:guid serves by default (an image or a playable video)
	PreviewMime string      `json:"previewMime"`
	Width       int16       `json:"width"`
	Height      int16       `json:"height"`
	Asset       clientAsset `json:"asset"` // every file of the asset, by role
}

// The client keeps its items between visits and asks only for what changed:
// /items?since=<cursor>. The cursor is the server's time when a response began; it
// comes as the stream's last line ({cursor, total}), so a stream cut short (a DB error
// after the 200 went out, a dropped connection) has none and the client keeps its old
// one. total: how many items are shown then — a client whose copy holds another count
// after applying the stream (a table left half-filled) starts again from nothing.
// The epoch (a header) names this database — another one (recreated, another library)
// means the client's copy is not a base for a delta, it fetches everything.
const (
	headerEpoch = "X-Sync-Epoch"
	epochKey    = "sync_epoch"
)

// endLine: the stream's last line, only after every item went out
type endLine struct {
	Cursor string `json:"cursor"`
	Total  int64  `json:"total"`
}

var syncEpoch string

// contractVersion: bumped when what an item carries changes (a new field of the
// asset): it is part of the epoch, so every client syncs from nothing once — a
// delta brings only changed items, the kept ones would never get the field.
// 2: asset.onDemand (Apple Photos).
const contractVersion = "2"

func RegisterItemsRoutes(segment string, e *echo.Echo, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	meta := model.MetaApi(model.NewProxy(logger))
	syncEpoch, _ = meta.GetMeta(epochKey)
	if syncEpoch == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		syncEpoch = hex.EncodeToString(b)
		if err := meta.SetMeta(epochKey, syncEpoch); err != nil {
			logger.Error("Sync epoch not kept", l.Error(err))
		}
	}

	userGroup := e.Group(segment)
	userGroup.GET("/", getItems) // all items
	userGroup.GET("", getItems)  // all items
}

// getItems: every shown item, newest first; ?since=<cursor>: only what changed since
// — changed shown items as usual, and {guid, removed} for the deleted or hidden ones
func getItems(c echo.Context) error {
	var since *time.Time
	if s := c.QueryParam("since"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "since: "+err.Error())
		}
		since = &t
	}
	c.Response().Header().Set(headerEpoch, syncEpoch+"."+contractVersion)
	// The count and the cursor at the same moment, just before the stream picks its
	// items: what the client holds after it matches the count
	cursor := time.Now().UTC().Format(time.RFC3339Nano)
	total, err := itemsProxy.CountItemsInStates(shownStates...)
	if err != nil {
		return err
	}
	return streamClientItems(c.Response().Writer, since, endLine{Cursor: cursor, Total: total})
}

// removedItem: a tombstone in a delta
type removedItem struct {
	Guid    string `json:"guid"`
	Removed bool   `json:"removed"`
}

func toClientItem(dbItem *dto.ItemDto, files []*dto.FileDto) clientItem {
	item := clientItem{
		Id:       dbItem.ID,
		Guid:     dbItem.Guid,
		Date:     dbItem.Date,
		MimeType: dbItem.MimeType,
		// Items shown before the cheap stage existed have no preview: the original
		PreviewMime: dbItem.PreviewMime,
		Asset:       toClientAsset(dbItem, files),
	}
	if item.PreviewMime == "" {
		item.PreviewMime = dbItem.MimeType
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

func streamClientItems(w http.ResponseWriter, since *time.Time, end endLine) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	flusher.Flush()

	encoder := json.NewEncoder(w)
	var err error
	if since == nil {
		err = itemsProxy.StreamAllItems(func(dbItem *dto.ItemDto, files []*dto.FileDto) error {
			if !shown(dbItem) {
				return nil
			}
			if err := encoder.Encode(toClientItem(dbItem, files)); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		})
	} else {
		err = itemsProxy.StreamItemsSince(*since, func(dbItem *dto.ItemDto, files []*dto.FileDto) error {
			var out any = toClientItem(dbItem, files)
			if dbItem.DeletedAt.Valid || !shown(dbItem) {
				out = removedItem{Guid: dbItem.Guid, Removed: true}
			}
			if err := encoder.Encode(out); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		})
	}
	if err != nil {
		return err
	}
	return encoder.Encode(end)
}

// shown: the client gets items it can display — Visible (a cheap preview) and Ready;
// Waiting (nothing viewable yet) and New stay hidden
func shown(item *dto.ItemDto) bool {
	return slices.Contains(shownStates, item.State)
}
