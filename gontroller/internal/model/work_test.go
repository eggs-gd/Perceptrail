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
	taken, _ := db.Take("render", []api.GUID{a.GUID})
	done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Lease: taken[0].Lease, Version: "v1", Input: "h1",
		Renditions: []dto.RenditionDto{{GUID: a.GUID, Size: 400, Format: "webp", Path: "r/a/v1-400.webp"}}})
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
	taken, _ = db.Take("render", []api.GUID{a.GUID})
	if done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Lease: taken[0].Lease, Version: "v1", Input: "h1"}); err != nil || done {
		t.Errorf("a stale result kept: %v %v", done, err)
	}
	if got := pass(t, db, "v1", 10); len(got) != 1 {
		t.Errorf("after a dropped result: due %v, want the item (its lease gone)", got)
	}
}

// A failure backs off — 1 min, 10 min, 1 h, then a day, again and again: never
// stopped for good (a newer tool may read the file); due again once its wait passed
func TestFail(t *testing.T) {
	db := openTest(t)
	a := workItem(t, db, "a.jpg", dto.Visible, time.Now(), "h1")
	row := func() dto.WorkDto {
		var row dto.WorkDto
		db.writer.db.Where("guid = ?", a.GUID).First(&row)
		return row
	}
	waits := []time.Duration{time.Minute, 10 * time.Minute, time.Hour, 24 * time.Hour, 24 * time.Hour, 24 * time.Hour}
	for i, wait := range waits {
		if err := db.Fail(dto.WorkFailed{Slug: "render", GUID: a.GUID, Version: "v1", Input: "h1", Err: "broken"}); err != nil {
			t.Fatal(err)
		}
		if got := pass(t, db, "v1", 10); len(got) != 0 {
			t.Fatalf("failure %d: due right away", i+1)
		}
		r := row()
		if r.Attempts != i+1 || r.Error != "broken" {
			t.Fatalf("failure %d: row %+v", i+1, r)
		}
		if left := time.Until(time.Unix(r.NextTry, 0)); left < wait-time.Minute || left > wait+time.Minute {
			t.Errorf("failure %d: next try in %v, want %v", i+1, left, wait)
		}
		db.writer.db.Model(&dto.WorkDto{}).Where("guid = ?", a.GUID).Update("next_try", 0) // the wait passed
		if got := pass(t, db, "v1", 10); len(got) != 1 {
			t.Fatalf("failure %d, its wait passed: due %v, want the item", i+1, got)
		}
	}
}

// A worker whose lease ran out and whose item another worker took: its late result
// changes nothing — the second worker's stands
func TestLateWorkerAfterLeaseLost(t *testing.T) {
	db := openTest(t)
	a := workItem(t, db, "a.jpg", dto.Visible, time.Now(), "h1")
	expire := func() { db.writer.db.Model(&dto.WorkDto{}).Where("guid = ?", a.GUID).Update("lease_until", 0) }

	first, _ := db.Take("render", []api.GUID{a.GUID})
	expire() // the first worker ran past its lease
	time.Sleep(time.Millisecond)
	second, _ := db.Take("render", []api.GUID{a.GUID})
	if len(first) != 1 || len(second) != 1 || first[0].Lease == second[0].Lease {
		t.Fatalf("leases %v %v", first, second)
	}
	if done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Lease: second[0].Lease, Version: "v1", Input: "h1"}); err != nil || !done {
		t.Fatalf("the second worker's finish: %v %v", done, err)
	}
	// The first one comes back late: neither its failure nor its result counts
	db.Fail(dto.WorkFailed{Slug: "render", GUID: a.GUID, Lease: first[0].Lease, Version: "v1", Input: "h1", Err: "late"})
	if done, _ := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Lease: first[0].Lease, Version: "v0", Input: "h1"}); done {
		t.Error("a late result was kept")
	}
	row, _ := db.Work(a.GUID, "render")
	if row.DoneAt == 0 || row.Error != "" || row.Version != "v1" {
		t.Errorf("the late worker changed the row: %+v", row)
	}
}

// Renditions make an item Ready; another pass of the cheap stage keeps it Ready (its
// renditions are for this fingerprint) and NeedsWork does not send it again; a new
// fingerprint sends it back to Waiting
func TestReadyStays(t *testing.T) {
	db := openTest(t)
	a := workItem(t, db, "a.heic", dto.Waiting, time.Now(), "h1")
	f, _ := db.GetFileByPath("/a.heic") // the file as the import leaves it: linked, a role
	f.LinkToItem(a.GUID)
	f.Role = dto.RoleOriginal
	db.UpdateFile(f)
	files := []*dto.FileDto{f}
	taken, _ := db.Take("render", []api.GUID{a.GUID})
	done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: a.GUID, Lease: taken[0].Lease, Version: "v1", Input: "h1",
		Renditions: []dto.RenditionDto{
			{GUID: a.GUID, Size: 1600, Format: "webp", W: 1600, H: 1200, Path: "r/a/v1/1600.webp"},
			{GUID: a.GUID, Size: 400, Format: "webp", W: 400, H: 300, Path: "r/a/v1/400.webp"}}})
	if err != nil || !done {
		t.Fatalf("finish: %v %v", done, err)
	}
	state := func() dto.ItemState { it, _ := db.GetItemByGUID(a.GUID); return it.State }
	if state() != dto.Ready {
		t.Fatalf("state %v, want Ready", state())
	}
	if rs, err := db.Renditions(a.GUID); err != nil || len(rs) != 2 || rs[0].Size != 400 {
		t.Errorf("renditions %+v %v, smallest first", rs, err)
	}

	it, _ := db.GetItemByGUID(a.GUID)
	if needs, _, err := db.NeedsWork(files, "", it.MetaHash); err != nil || needs {
		t.Errorf("a rendered item sent to the cheap stage again: %v %v", needs, err)
	}
	if _, err := db.Publish(it); err != nil { // another pass (a perceptor's rework)
		t.Fatal(err)
	}
	if state() != dto.Ready {
		t.Errorf("another pass: state %v, want Ready", state())
	}

	it, _ = db.GetItemByGUID(a.GUID)
	it.HashShort = "h2" // the file changed
	db.UpdateItem(it)
	db.Publish(it)
	if state() != dto.Waiting {
		t.Errorf("a new fingerprint: state %v, want Waiting", state())
	}
}

// A new render replaces an item's renditions; Prune takes what a deleted item had;
// what is shown stays
func TestPrune(t *testing.T) {
	db := openTest(t)
	finish := func(item *dto.ItemDto, version string) {
		t.Helper()
		taken, _ := db.Take("render", []api.GUID{item.GUID})
		path := "r/" + string(item.GUID) + "/" + version + "/400.webp"
		done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: item.GUID, Lease: taken[0].Lease, Version: version, Input: item.HashShort,
			Renditions: []dto.RenditionDto{{GUID: item.GUID, Size: 400, Format: "webp", Path: path}}})
		if err != nil || !done {
			t.Fatalf("finish %s: %v %v", version, done, err)
		}
	}
	kept := workItem(t, db, "kept.jpg", dto.Visible, time.Now(), "h1")
	gone := workItem(t, db, "gone.jpg", dto.Visible, time.Now(), "h2")
	finish(kept, "v1")
	finish(kept, "v2") // another render: it replaces v1's
	finish(gone, "v2")
	gone, _ = db.GetItemByGUID(gone.GUID)
	if err := db.DeleteItem(gone); err != nil {
		t.Fatal(err)
	}

	if n, err := db.Prune(); err != nil || n != 2 {
		t.Errorf("pruned %d rows (%v), want 2: gone's rendition and its work", n, err)
	}
	paths, _ := db.RenditionPaths()
	if want := "r/" + string(kept.GUID) + "/v2/400.webp"; len(paths) != 1 || paths[0] != want {
		t.Errorf("paths %v, want only %s", paths, want)
	}
	if rs, _ := db.Renditions(kept.GUID); len(rs) != 1 || rs[0].Path != "r/"+string(kept.GUID)+"/v2/400.webp" {
		t.Errorf("shown %+v", rs)
	}
}
