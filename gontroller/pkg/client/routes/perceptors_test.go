package routes

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins/exif_core/date"
	"perceptrail/gontroller/pkg/plugins/exif_core/size"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"

	"github.com/labstack/echo/v4"
)

// The perceptors the client is given, each with its view; the date's order over the
// shown items: newest first with its sections
func TestPerceptorsRoutes(t *testing.T) {
	e := echo.New()
	RegisterPerceptorsRoutes(e, []api.Perceptor{date.Perceptor, size.Perceptor}, nil,
		l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))

	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	for _, it := range []*dto.ItemDto{
		{Guid: "nav-old", State: dto.Ready, Date: at("2025-03-01T10:00:00Z"), DateSource: "tag"},
		{Guid: "nav-new", State: dto.Visible, Date: at("2026-09-01T10:00:00Z"), DateSource: "tag"},
		{Guid: "nav-hidden", State: dto.Waiting, Date: at("2026-09-02T10:00:00Z"), DateSource: "tag"},
	} {
		if _, err := itemsProxy.UpdateItem(it); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/perceptors", nil))
	var views []clientPerceptor
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Name != date.Perceptor.Name() || views[0].Title != "Date" ||
		views[1].Title != "Size" || !strings.HasPrefix(views[0].Icon, "<svg") {
		t.Fatalf("perceptors %+v, want date, then size", views)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/"+date.Perceptor.Name()+"/order", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("order: %d", rec.Code)
	}
	var got []string
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		var en clientEntry
		if err := json.Unmarshal(sc.Bytes(), &en); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(en.Guid, "nav-") {
			continue // items of other tests
		}
		s := ""
		for _, sec := range en.Sections {
			s += "|" + sec.Label
		}
		got = append(got, en.Guid+s)
	}
	if want := "nav-new|2026|September nav-old|2025|March"; strings.Join(got, " ") != want {
		t.Errorf("order %q, want %q", strings.Join(got, " "), want)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/nope/order", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown perceptor: %d", rec.Code)
	}
}
