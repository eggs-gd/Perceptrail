package route_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/perceptor/date"
	"perceptrail/gontroller/internal/perceptor/size"
)

// The perceptors the client is given, each with its view; the date's order over the
// shown items: newest first with its sections
func TestPerceptorsRoutes(t *testing.T) {
	e := server(nil, date.Perceptor, size.Perceptor)

	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	for _, it := range []*dto.ItemDto{
		{Guid: "nav-old", State: dto.Ready, Date: at("2025-03-01T10:00:00Z"), DateSource: "tag"},
		{Guid: "nav-new", State: dto.Visible, Date: at("2026-09-01T10:00:00Z"), DateSource: "tag"},
		{Guid: "nav-hidden", State: dto.Waiting, Date: at("2026-09-02T10:00:00Z"), DateSource: "tag"},
	} {
		if _, err := testDB.UpdateItem(it); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/perceptors", nil))
	var views []struct{ Slug, Title, Icon string }
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Slug != "date" || views[0].Title != "Date" ||
		views[1].Title != "Size" || !strings.HasPrefix(views[0].Icon, "<svg") {
		t.Fatalf("perceptors %+v, want date, then size", views)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/date/order", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("order: %d", rec.Code)
	}
	var got []string
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		var en struct {
			Guid     string
			Sections []struct{ Label string }
		}
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

// A view is reached by its slug: a taken one is not given to the client
func TestPerceptorsSlugTaken(t *testing.T) {
	e := server(nil, date.Perceptor, date.Perceptor, size.Perceptor)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/perceptors", nil))
	var views []struct{ Slug, Title, Icon string }
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Slug != "date" || views[1].Slug != "size" {
		t.Errorf("views %+v, want date once, then size", views)
	}
}

// The info panel: what each perceptor knows about one item; one that says nothing is
// left out (Size: an item without a size)
func TestPerceptorsInfo(t *testing.T) {
	e := server(nil, date.Perceptor, size.Perceptor)
	at, _ := time.Parse(time.RFC3339, "2025-09-14T05:00:00Z")
	if _, err := testDB.UpdateItem(&dto.ItemDto{Guid: "info-1", State: dto.Visible, Date: at, DateSource: "tag", DateOffset: 180}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/info-1/info", nil))
	var info []struct {
		Slug  string
		Facts []struct{ Value string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if len(info) != 1 || info[0].Slug != "date" || info[0].Facts[0].Value != "14 Sep 2025, 08:00 +03:00" {
		t.Errorf("info %+v, want the date only, in the shot's zone", info)
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/nope/info", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", rec.Code)
	}
}
