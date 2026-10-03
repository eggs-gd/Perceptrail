package importer

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model/dto"
)

const (
	appleEdited = "A1111111-0000-0000-0000-00000000000A"
	appleCloud  = "B2222222-0000-0000-0000-00000000000B"
	appleLive   = "C3333333-0000-0000-0000-00000000000C"
)

// photosLibrary writes a minimal Apple Photos library under root: ZASSET rows and
// files per the layout. Returns the bundle and a func to change ZASSET.
func photosLibrary(t *testing.T, root string) (string, func(query string, args ...any)) {
	t.Helper()
	bundle := filepath.Join(root, "Photos Library.photoslibrary")
	files := []string{
		"originals/A/" + appleEdited + ".heic",
		"resources/renders/A/" + appleEdited + "_1_201_a.jpeg",
		"resources/derivatives/A/" + appleEdited + "_1_102_o.jpeg",
		"resources/derivatives/B/" + appleCloud + "_1_105_c.jpeg",
		"resources/derivatives/masters/B/" + appleCloud + "_4_5005_c.jpeg",
		"originals/C/" + appleLive + ".heic",
		"originals/C/" + appleLive + "_3.mov",
		"resources/derivatives/C/" + appleLive + "_1_102_o.jpeg",
	}
	for _, f := range files {
		write(t, filepath.Join(bundle, f), f)
	}
	os.MkdirAll(filepath.Join(bundle, "database"), 0o755)
	db, err := sql.Open("sqlite3", filepath.Join(bundle, "database", "Photos.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE ZASSET (Z_PK INTEGER PRIMARY KEY, ZUUID VARCHAR, ZDIRECTORY VARCHAR,
		ZFILENAME VARCHAR, ZTRASHEDSTATE INTEGER, ZHIDDEN INTEGER, ZKIND INTEGER, ZPLAYBACKSTYLE INTEGER, ZDURATION FLOAT, ZDATECREATED TIMESTAMP,
		ZWIDTH INTEGER, ZHEIGHT INTEGER, ZLATITUDE FLOAT, ZLONGITUDE FLOAT)`)
	exec(`CREATE TABLE ZADDITIONALASSETATTRIBUTES (Z_PK INTEGER PRIMARY KEY, ZASSET INTEGER, ZTIMEZONEOFFSET INTEGER)`)
	for i, a := range [][3]string{
		{appleEdited, "A", appleEdited + ".heic"},
		{appleCloud, "B", appleCloud + ".jpeg"},
		{appleLive, "C", appleLive + ".heic"},
	} {
		exec(`INSERT INTO ZASSET (Z_PK, ZUUID, ZDIRECTORY, ZFILENAME, ZTRASHEDSTATE, ZHIDDEN, ZDATECREATED,
			ZWIDTH, ZHEIGHT, ZLATITUDE, ZLONGITUDE) VALUES (?,?,?,?,0,0, 758992569, 3024, 4032, -180, -180)`, i+1, a[0], a[1], a[2])
		exec(`INSERT INTO ZADDITIONALASSETATTRIBUTES (ZASSET, ZTIMEZONEOFFSET) VALUES (?, 7200)`, i+1)
	}
	return bundle, exec
}

func TestApplePhotosLibrary(t *testing.T) {
	root := t.TempDir()
	bundle, exec := photosLibrary(t, root)

	scan(t, root)
	check := func(guid, preview string) *dto.ItemDto {
		t.Helper()
		item, err := itemsProxy.GetItemByGuid(guid)
		if err != nil {
			t.Fatalf("no item %s: %v", guid, err)
		}
		if item.State != dto.Visible || filepath.Base(item.PreviewPath) != preview {
			t.Errorf("%s: state %d, preview %q, want Visible, %q", guid, item.State, filepath.Base(item.PreviewPath), preview)
		}
		return item
	}
	check(appleEdited, appleEdited+"_1_201_a.jpeg") // the edit
	check(appleCloud, appleCloud+"_1_105_c.jpeg")   // cloud-only: the biggest derivative
	check(appleLive, appleLive+"_1_102_o.jpeg")     // HEIC not viewable: Apple's JPEG
	if f, _ := filesProxy.GetFileByPath(filepath.Join(bundle, "originals/C/"+appleLive+"_3.mov")); f.LinkedTo != appleLive {
		t.Errorf("a file of the asset is not linked to its key: %+v", f)
	}

	if got := scan(t, root); len(got) != 0 {
		t.Errorf("nothing changed, processed %v", got)
	}

	// Photos downloads the cloud-only original: the same item, the original shown
	write(t, filepath.Join(bundle, "originals/B/"+appleCloud+".jpeg"), "the original")
	scan(t, root)
	if item := check(appleCloud, appleCloud+".jpeg"); filepath.Base(item.Path) != appleCloud+".jpeg" {
		t.Errorf("cloud-only: main %s, want the original", item.Path)
	}

	// Moved to the trash in Photos: the item goes
	exec(`UPDATE ZASSET SET ZTRASHEDSTATE = 1 WHERE ZUUID = ?`, appleEdited)
	scan(t, root)
	assertNoItem(t, appleEdited)
}

// The DB is the truth: the date and size come from it (the files have no EXIF
// here at all); a date corrected in Photos reaches the item on the next walk
func TestAppleMetadataFromDB(t *testing.T) {
	root := t.TempDir()
	_, exec := photosLibrary(t, root)
	scan(t, root)

	item, err := itemsProxy.GetItemByGuid(appleCloud)
	if err != nil {
		t.Fatal(err)
	}
	local := item.Date.In(time.FixedZone("", item.DateOffset*60)).Format(time.RFC3339)
	if local != "2025-01-19T17:16:09+02:00" || item.Size.W != 3024 || item.Size.H != 4032 {
		t.Errorf("from the DB: date %s, size %v", local, item.Size)
	}

	// Corrected in Photos by a day; no file changed
	exec(`UPDATE ZASSET SET ZDATECREATED = ZDATECREATED - 86400 WHERE ZUUID = ?`, appleCloud)
	if got := scan(t, root); len(got) != 1 {
		t.Fatalf("processed %v, want the one asset whose DB metadata changed", got)
	}
	item, _ = itemsProxy.GetItemByGuid(appleCloud)
	if got := item.Date.In(time.FixedZone("", item.DateOffset*60)).Format(time.RFC3339); got != "2025-01-18T17:16:09+02:00" {
		t.Errorf("corrected date %s", got)
	}
}

// One asset asked again on demand (Refresh): processed even if nothing changed (it
// then always reaches the end, where its waiters hear it), its item kept
func TestRefreshProcessesAsked(t *testing.T) {
	root := t.TempDir()
	photosLibrary(t, root)
	h := newHarness(t)
	h.scan(root)
	if got := h.scan(root); len(got) != 0 {
		t.Fatalf("nothing changed, processed %v", got)
	}
	// Asked again on demand: processed though nothing changed, its item kept
	guid := appleEdited
	if got := h.refresh(appleEdited); len(got) != 1 {
		t.Fatalf("refresh processed %v, want the asset", got)
	}
	if item, err := itemsProxy.GetItemByGuid(guid); err != nil || item.State != dto.Visible {
		t.Errorf("after refresh: %+v", item)
	}
	if got := h.refresh("no-such-asset"); len(got) != 0 {
		t.Errorf("an unknown asset processed %v", got)
	}
}
