package importer_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"perceptrail/gontroller/internal/importer"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
	"github.com/eggs-gd/perceplib/api"
)

// A JPEG imported alone is an item; when its RAW appears, the RAW is the source:
// the JPEG's item goes, the JPEG becomes the RAW's sidecar (a preview later)
func TestSourceBecomesMain(t *testing.T) {
	root := t.TempDir()
	jpeg, raw := filepath.Join(root, "D.JPG"), filepath.Join(root, "D.NEF")
	write(t, jpeg, "derivative")
	scan(t, root)
	oldGuid := itemAt(t, jpeg).GUID

	write(t, raw, "source")
	scan(t, root)
	assertNoItem(t, oldGuid)
	item := itemAt(t, raw)
	if f, _ := testDB.GetFileByPath(jpeg); f.LinkedTo != item.GUID {
		t.Errorf("the JPEG is not linked to the RAW: %s vs %s", f.LinkedTo, item.GUID)
	}
}

// A move keeps the item (its GUID); the cheap stage runs again for it, so its
// preview follows the file
func TestMovedKeepsItemAndPreview(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.jpg")
	write(t, a, "moving")
	scan(t, root)
	guid := itemAt(t, a).GUID

	moved := filepath.Join(root, "sub", "a.jpg")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(a, moved); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	if item := itemAt(t, moved); item.GUID != guid || item.State != dto.Visible || item.PreviewPath != moved {
		t.Errorf("moved: %+v", item)
	}
}

// The fingerprint changed (hashVersion): every group is identified once more and
// keeps its item; then the walk is idle again
func TestFingerprintChangeReidentifies(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a.jpg"), filepath.Join(root, "b.jpg")
	write(t, a, "first")
	write(t, b, "second")
	scan(t, root)
	guid := itemAt(t, a).GUID

	before := itemAt(t, a)
	if _, err := testDB.ClearHashes(); err != nil {
		t.Fatal(err)
	}
	if item := itemAt(t, a); !item.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("clearing the hashes moved updated_at (a client's delta would stream the library)")
	}
	if got := scan(t, root); len(got) != 2 {
		t.Fatalf("re-identified %v, want both", got)
	}
	if item := itemAt(t, a); item.GUID != guid || item.HashShort == "" || item.State != dto.Visible {
		t.Errorf("after: %+v", item)
	}
	if got := scan(t, root); len(got) != 0 {
		t.Errorf("walked again: %v", got)
	}
}

// An item marked for rework (a perceptor without its row) is processed once more, its
// files unchanged; publishing clears the mark
func TestReworkReprocessesOnce(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.jpg")
	write(t, a, "rework me")
	scan(t, root)
	guid := itemAt(t, a).GUID

	if n, err := testDB.MarkRework([]api.GUID{guid}); err != nil || n != 1 {
		t.Fatalf("marked %d, %v", n, err)
	}
	if got := scan(t, root); len(got) != 1 {
		t.Fatalf("processed %v, want the marked one", got)
	}
	if item := itemAt(t, a); item.Rework || item.GUID != guid {
		t.Errorf("after: %+v", item)
	}
	if got := scan(t, root); len(got) != 0 {
		t.Errorf("walked again: %v", got)
	}
}

// Not media: remembered as ignored, not read again on the next walk
func TestNotMediaIgnored(t *testing.T) {
	root := t.TempDir()
	notes := filepath.Join(root, "notes.txt")
	write(t, notes, "hello")
	scan(t, root)
	if f, err := testDB.GetFileByPath(notes); err != nil || !f.IsIgnored() {
		t.Fatalf("not ignored: %+v %v", f, err)
	}
	row, _ := testDB.GetFileByPath(notes) // as the walk gives it: unchanged
	if needs, _, err := testDB.NeedsWork([]*dto.FileDto{row}, "", ""); err != nil || needs {
		t.Errorf("an unchanged ignored group needs work: %v %v", needs, err)
	}
}

// A RAW's JPEG is what its pixels are read from too: the JPEG edited alone changes
// the item's pixels' fingerprint (render is due again), not its identity; an item of
// one file has its own fingerprint as both
func TestPixelsHashFollowsThePreview(t *testing.T) {
	root := t.TempDir()
	raw, jpeg, lone := filepath.Join(root, "D.NEF"), filepath.Join(root, "D.JPG"), filepath.Join(root, "E.JPG")
	write(t, raw, "source")
	write(t, jpeg, "derivative")
	write(t, lone, "alone")
	scan(t, root)
	before := itemAt(t, raw)
	if before.PreviewPath != jpeg || before.PixelsHash == "" || before.PixelsHash == before.HashShort {
		t.Fatalf("RAW + JPEG: preview %q, pixels %q, hash %q", before.PreviewPath, before.PixelsHash, before.HashShort)
	}
	if item := itemAt(t, lone); item.PixelsHash != item.HashShort {
		t.Errorf("one file: pixels %q, hash %q", item.PixelsHash, item.HashShort)
	}

	write(t, jpeg, "derivative, edited")
	scan(t, root)
	after := itemAt(t, raw)
	if after.GUID != before.GUID || after.HashShort != before.HashShort || after.PixelsHash == before.PixelsHash {
		t.Errorf("JPEG edited: guid %v→%v, hash %q→%q, pixels %q→%q",
			before.GUID, after.GUID, before.HashShort, after.HashShort, before.PixelsHash, after.PixelsHash)
	}
}

// The RAW is deleted, its JPEG stays: the JPEG becomes the item in the same walk
// (it was linked to the RAW's item, which the walk deletes)
func TestFormerMainGone(t *testing.T) {
	root := t.TempDir()
	raw, jpeg := filepath.Join(root, "D.NEF"), filepath.Join(root, "D.JPG")
	write(t, raw, "source")
	write(t, jpeg, "derivative")
	scan(t, root)
	rawGuid := itemAt(t, raw).GUID

	if err := os.Remove(raw); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	assertNoItem(t, rawGuid)
	if item := itemAt(t, jpeg); item.State != dto.Visible {
		t.Errorf("the JPEG is not an item after one walk: %+v", item)
	}
}

// An exiftool that cannot start fails the pass with its reason — the server goes
// on (the next pass tries again), nothing is published, no process is left
func TestExiftoolThatCannotStart(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.jpg")
	write(t, a, "never read")
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	err := importer.New(pass{root: root, cache: t.TempDir(), exiftool: filepath.Join(root, "no-exiftool")}, testDB, logger).Pass(t.Context())
	if err == nil || !strings.Contains(err.Error(), "can't start") {
		t.Fatalf("pass error %v, want exiftool's start failure", err)
	}
	if _, err := testDB.GetItemByPath(a); err == nil {
		t.Errorf("an item published without its metadata")
	}
	// The next pass, with exiftool there, does the work
	if got := scan(t, root); !slices.Equal(got, []string{a}) {
		t.Errorf("next pass published %v", got)
	}
}
