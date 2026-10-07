package model

import (
	"slices"
	"testing"
	"time"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// workItem: an item in a state, of a date, with a fingerprint
func workItem(t *testing.T, db *Proxy, name string, state dto.ItemState, date time.Time, hash string) *dto.ItemDto {
	t.Helper()
	f, err := db.CreateFile(dto.ItemEntry{Path: "/" + name, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateItem(f)
	if err != nil {
		t.Fatal(err)
	}
	item.State, item.Date, item.HashShort = state, date, hash
	if _, err := db.UpdateItem(item); err != nil {
		t.Fatal(err)
	}
	return item
}

// pass: every item due, page by page of n
func pass(t *testing.T, db *Proxy, version string, n int) []api.GUID {
	t.Helper()
	var got []api.GUID
	var cursor dto.Cursor
	for !cursor.Over() {
		page, next, err := db.Due("render", version, cursor, n)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range page {
			got = append(got, it.GUID)
		}
		cursor = next
	}
	return got
}

// Waiting items first, then the shown ones newest first; nothing else is due; a page
// of one walks the same order
func TestDueOrder(t *testing.T) {
	db := openTest(t)
	day := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	old := workItem(t, db, "old.jpg", dto.Visible, day, "h1")
	waiting := workItem(t, db, "raw.heic", dto.Waiting, day, "h2")
	recent := workItem(t, db, "new.jpg", dto.Visible, day.Add(time.Hour), "h3")
	ready := workItem(t, db, "done.jpg", dto.Ready, day.Add(-time.Hour), "h4")
	workItem(t, db, "dirty.jpg", dto.Dirty, day, "h5")
	gone := workItem(t, db, "gone.jpg", dto.Visible, day, "h6")
	if err := db.DeleteItem(gone); err != nil {
		t.Fatal(err)
	}

	want := []api.GUID{waiting.GUID, recent.GUID, old.GUID, ready.GUID}
	if got := pass(t, db, "v1", 10); !slices.Equal(got, want) {
		t.Errorf("one page: %v, want %v", got, want)
	}
	if got := pass(t, db, "v1", 1); !slices.Equal(got, want) {
		t.Errorf("pages of one: %v, want %v", got, want)
	}
}

// Taken items are not due; a second take of them gets nothing
func TestTake(t *testing.T) {
	db := openTest(t)
	a := workItem(t, db, "a.jpg", dto.Visible, time.Now(), "h")
	if taken, err := db.Take("render", []api.GUID{a.GUID}); err != nil || len(taken) != 1 {
		t.Fatalf("take: %v %v", taken, err)
	}
	if got := pass(t, db, "v1", 10); len(got) != 0 {
		t.Errorf("a taken item is due: %v", got)
	}
	if taken, _ := db.Take("render", []api.GUID{a.GUID}); len(taken) != 0 {
		t.Errorf("taken twice: %v", taken)
	}
}

// Done is not due — until the version or the fingerprint changes; a result for a
// fingerprint that changed meanwhile is dropped and the item stays due
func TestFinish(t *testing.T) {
	db := openTest(t)
	a := workItem(t, db, "a.jpg", dto.Visible, time.Now(), "h1")
	db.Take("render", []api.GUID{a.GUID})
	done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Version: "v1", Input: "h1",
		Renditions: []dto.RenditionDto{{GUID: a.GUID, Version: "v1", Size: 400, Format: "webp", Path: "r/a/v1-400.webp"}}})
	if err != nil || !done {
		t.Fatalf("finish: %v %v", done, err)
	}
	if got := pass(t, db, "v1", 10); len(got) != 0 {
		t.Errorf("done and due: %v", got)
	}
	if got := pass(t, db, "v2", 10); len(got) != 1 {
		t.Errorf("a new version: due %v, want the item", got)
	}
	var kept int64
	db.writer.db.Model(&dto.RenditionDto{}).Where("guid = ?", a.GUID).Count(&kept)
	if kept != 1 {
		t.Errorf("renditions %d, want 1", kept)
	}

	a.HashShort = "h2" // the file changed while it was rendered
	db.UpdateItem(a)
	db.Take("render", []api.GUID{a.GUID})
	if done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Version: "v1", Input: "h1"}); err != nil || done {
		t.Errorf("a stale result kept: %v %v", done, err)
	}
	if got := pass(t, db, "v1", 10); len(got) != 1 {
		t.Errorf("after a dropped result: due %v, want the item (its lease gone)", got)
	}
}

// A failure backs off; maxAttempts in a row for the same version and input and the
// item waits for a new one
func TestFail(t *testing.T) {
	db := openTest(t)
	a := workItem(t, db, "a.jpg", dto.Visible, time.Now(), "h1")
	retry := func() { // the back-off passed
		db.writer.db.Model(&dto.WorkDto{}).Where("guid = ?", a.GUID).Update("next_try", 0)
	}
	for i := 1; i <= maxAttempts; i++ {
		if err := db.Fail(dto.WorkFailed{Slug: "render", GUID: a.GUID, Version: "v1", Input: "h1", Err: "broken"}); err != nil {
			t.Fatal(err)
		}
		if got := pass(t, db, "v1", 10); len(got) != 0 {
			t.Fatalf("failure %d: due right away", i)
		}
		retry()
	}
	if got := pass(t, db, "v1", 10); len(got) != 0 {
		t.Errorf("after %d failures: still due", maxAttempts)
	}
	if got := pass(t, db, "v2", 10); len(got) != 1 {
		t.Errorf("a new version: due %v, want the item", got)
	}
	var row dto.WorkDto
	db.writer.db.Where("guid = ?", a.GUID).First(&row)
	if row.Attempts != maxAttempts || row.Error != "broken" {
		t.Errorf("row %+v", row)
	}
}
