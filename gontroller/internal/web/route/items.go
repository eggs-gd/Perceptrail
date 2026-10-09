package route

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"perceptrail/gontroller/internal/model/dto"
	"slices"
	"time"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"

	"github.com/labstack/echo/v4"
)

type clientItem struct {
	Id       uint      `json:"id"`
	GUID     api.GUID  `json:"guid"`
	Date     time.Time `json:"date"`
	MimeType string    `json:"mimeType"`
	// What /assets/:guid serves by default (an image or a playable video)
	PreviewMime  string      `json:"previewMime"`
	PreviewColor string      `json:"previewColor,omitempty"`
	Width        int16       `json:"width"`
	Height       int16       `json:"height"`
	Asset        clientAsset `json:"asset"` // every file of the asset, by role
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

// contractVersion: bumped when what an item carries changes (a new field of the
// asset): it is part of the epoch, so every client syncs from nothing once — a
// delta brings only changed items, the kept ones would never get the field.
// 2: asset.onDemand (Apple Photos); 3: asset.onDemand.original; 4: the original
// for every Photos item (the biggest of what is seen, the edit); 5: the on-demand
// URLs carry the version (?v=); 6: asset.full (the full size of what is seen); 7:
// our renditions in stills and motion — and a client that kept an asset.motion of
// null (a build of #37 sent one) gets a clean copy; 8: previewColor; 9:
// previewColor for video previews too.
const contractVersion = "9"

// removedItem: a tombstone in a delta
type removedItem struct {
	GUID    api.GUID `json:"guid"`
	Removed bool     `json:"removed"`
}

func (r *routes) registerItems(segment string, e *echo.Echo) {
	r.epoch, _ = r.db.GetMeta(epochKey)
	if r.epoch == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		r.epoch = hex.EncodeToString(b)
		if err := r.db.SetMeta(epochKey, r.epoch); err != nil {
			r.logger.Error("Sync epoch not kept", l.Error(err))
		}
	}

	userGroup := e.Group(segment)
	userGroup.GET("/", r.getItems) // all items
	userGroup.GET("", r.getItems)  // all items
}

// r.getItems: every shown item, newest first; ?since=<cursor>: only what changed since
// — changed shown items as usual, and {guid, removed} for the deleted or hidden ones
func (r *routes) getItems(c echo.Context) error {
	var since *time.Time
	if s := c.QueryParam("since"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "since: "+err.Error())
		}
		since = &t
	}
	c.Response().Header().Set(headerEpoch, r.epoch+"."+contractVersion)
	// The count and the cursor at the same moment, just before the stream picks its
	// items: what the client holds after it matches the count
	cursor := time.Now().UTC().Format(time.RFC3339Nano)
	total, err := r.db.CountItemsInStates(shownStates...)
	if err != nil {
		return err
	}
	return r.streamClientItems(c.Response().Writer, since, endLine{Cursor: cursor, Total: total})
}

func (r *routes) toClientItem(stored dto.StoredItem) clientItem {
	dbItem := stored.Item
	asset := toClientAsset(dbItem, stored.Files, r.of(dbItem))
	withRenditions(&asset, dbItem, stored.Renditions)
	item := clientItem{
		Id:       dbItem.ID,
		GUID:     dbItem.GUID,
		Date:     dbItem.Date,
		MimeType: dbItem.MimeType,
		// Items shown before the cheap stage existed have no preview: the original
		PreviewMime:  dbItem.PreviewMime,
		PreviewColor: dbItem.PreviewColor,
		Asset:        asset,
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

func (r *routes) streamClientItems(w http.ResponseWriter, since *time.Time, end endLine) error {
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
		err = r.db.StreamAllItems(func(stored dto.StoredItem) error {
			if !shown(stored.Item) {
				return nil
			}
			if err := encoder.Encode(r.toClientItem(stored)); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		})
	} else {
		err = r.db.StreamItemsSince(*since, func(stored dto.StoredItem) error {
			var out any = r.toClientItem(stored)
			if stored.Item.DeletedAt.Valid || !shown(stored.Item) {
				out = removedItem{GUID: stored.Item.GUID, Removed: true}
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
