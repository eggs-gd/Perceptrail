package date

import (
	"context"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

func TestOrder(t *testing.T) {
	kyiv := 120 // +02:00 in winter, minutes
	item := func(guid, utc string, offset int) *dto.ItemDto {
		i := &dto.ItemDto{Guid: guid, DateOffset: offset}
		if utc != "" {
			i.Date, _ = time.Parse(time.RFC3339, utc)
			i.DateSource = "DateTimeOriginal"
		}
		return i
	}
	items := []api.ItemDataProvider{
		item("old", "2024-05-01T10:00:00Z", 0),
		item("nodate", "", 0),
		// 31 Dec 23:30 in Kyiv is 21:30 UTC: still December 2025 there
		item("nye", "2025-12-31T21:30:00Z", kyiv),
		// The same instant an hour further east (+03:00) is 1 Jan 00:30: January 2026
		item("ny", "2025-12-31T21:30:00Z", kyiv+60),
		item("sep", "2026-09-15T08:00:00Z", 0),
		item("sep2", "2026-09-20T08:00:00Z", 0),
	}
	got, err := Perceptor.Order(context.Background(), "", items)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		guid    string
		section string // "level:label", "" = none
	}{
		{"sep2", "0:2026"},
		{"sep", ""},
		{"ny", "1:January"},
		{"nye", "0:2025"},
		{"old", "0:2024"},
		{"nodate", "0:No date"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, w := range want {
		s := ""
		if got[i].Section != nil {
			s = string(rune('0'+got[i].Section.Level)) + ":" + got[i].Section.Label
		}
		if got[i].Guid != w.guid || s != w.section {
			t.Errorf("%d: %s %q, want %s %q", i, got[i].Guid, s, w.guid, w.section)
		}
	}
}
