package routes

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"

	"github.com/labstack/echo/v4"
)

// The delta: after a full fetch, ?since=<its cursor> gives only what changed — a
// changed or new shown item as an item, a deleted or hidden one as removed; the
// epoch stays the same
func TestItemsDelta(t *testing.T) {
	e := echo.New()
	RegisterItemsRoutes("/items", e, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))

	put := func(it *dto.ItemDto) *dto.ItemDto {
		t.Helper()
		out, err := itemsProxy.UpdateItem(it)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	keep := put(&dto.ItemDto{Guid: "d-keep", State: dto.Visible})
	change := put(&dto.ItemDto{Guid: "d-change", State: dto.Visible})
	gone := put(&dto.ItemDto{Guid: "d-gone", State: dto.Visible})
	hide := put(&dto.ItemDto{Guid: "d-hide", State: dto.Visible})
	_ = keep

	get := func(query string) (map[string]bool, http.Header) {
		t.Helper()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items"+query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", query, rec.Code)
		}
		got := map[string]bool{} // guid → removed
		sc := bufio.NewScanner(rec.Body)
		for sc.Scan() {
			var line struct {
				Guid    string `json:"guid"`
				Removed bool   `json:"removed"`
			}
			if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(line.Guid, "d-") {
				got[line.Guid] = line.Removed
			}
		}
		return got, rec.Header()
	}

	full, h := get("")
	if len(full) != 4 || h.Get(headerEpoch) == "" || h.Get(headerCursor) == "" {
		t.Fatalf("full: %v, headers %v", full, h)
	}
	cursor := h.Get(headerCursor)
	time.Sleep(10 * time.Millisecond)

	change.MimeType = "image/jpeg"
	put(change)
	if err := itemsProxy.DeleteItem(gone); err != nil {
		t.Fatal(err)
	}
	hide.State = dto.Waiting
	put(hide)
	put(&dto.ItemDto{Guid: "d-new", State: dto.Visible})

	delta, h2 := get("?since=" + url.QueryEscape(cursor))
	want := map[string]bool{"d-change": false, "d-new": false, "d-gone": true, "d-hide": true}
	if len(delta) != len(want) {
		t.Errorf("delta %v, want %v", delta, want)
	}
	for guid, removed := range want {
		if r, ok := delta[guid]; !ok || r != removed {
			t.Errorf("%s: in delta %v removed %v, want removed %v", guid, ok, r, removed)
		}
	}
	if h2.Get(headerEpoch) != h.Get(headerEpoch) {
		t.Error("the epoch changed")
	}
}
