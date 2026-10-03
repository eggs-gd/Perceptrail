package importer

import (
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/importer/gate"
	"perceptrail/gontroller/pkg/importer/walk"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"
)

// A JPEG imported alone is an item; when its RAW appears, the RAW is the source:
// the JPEG's item goes, the JPEG becomes the RAW's sidecar (a preview later)
func TestSourceBecomesMain(t *testing.T) {
	root := t.TempDir()
	jpeg, raw := filepath.Join(root, "D.JPG"), filepath.Join(root, "D.NEF")
	write(t, jpeg, "derivative")
	scan(t, root)
	oldGuid := itemAt(t, jpeg).Guid

	write(t, raw, "source")
	scan(t, root)
	assertNoItem(t, oldGuid)
	item := itemAt(t, raw)
	if f, _ := filesProxy.GetFileByPath(jpeg); f.LinkedTo != item.Guid {
		t.Errorf("the JPEG is not linked to the RAW: %s vs %s", f.LinkedTo, item.Guid)
	}
}

// A move keeps the item (its GUID); the cheap stage runs again for it, so its
// preview follows the file
func TestMovedKeepsItemAndPreview(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.jpg")
	write(t, a, "moving")
	scan(t, root)
	guid := itemAt(t, a).Guid

	moved := filepath.Join(root, "sub", "a.jpg")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(a, moved); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	if item := itemAt(t, moved); item.Guid != guid || item.State != dto.Visible || item.PreviewPath != moved {
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
	guid := itemAt(t, a).Guid

	if _, err := testDB.ClearHashes(); err != nil {
		t.Fatal(err)
	}
	if got := scan(t, root); len(got) != 2 {
		t.Fatalf("re-identified %v, want both", got)
	}
	if item := itemAt(t, a); item.Guid != guid || item.HashShort == "" || item.State != dto.Visible {
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
	guid := itemAt(t, a).Guid

	if n, err := testDB.MarkRework([]string{guid}); err != nil || n != 1 {
		t.Fatalf("marked %d, %v", n, err)
	}
	if got := scan(t, root); len(got) != 1 {
		t.Fatalf("processed %v, want the marked one", got)
	}
	if item := itemAt(t, a); item.Rework || item.Guid != guid {
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
	if f, err := filesProxy.GetFileByPath(notes); err != nil || !f.IsIgnored() {
		t.Fatalf("not ignored: %+v %v", f, err)
	}
	gate := gate.NewGate(testDB, nil, &walk.Result{})
	if _, err := gate.Decorate(providers.Group{Asset: providers.Asset{Files: []*dto.FileDto{{ItemEntry: statEntry(t, notes)}}}}); err == nil {
		t.Error("the gate let an unchanged ignored group through")
	}
}

func statEntry(t *testing.T, path string) dto.ItemEntry {
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return dto.ItemEntry{Path: path, Name: filepath.Base(path), Size: info.Size(), ModTime: info.ModTime()}
}

// The RAW is deleted, its JPEG stays: the JPEG becomes the item in the same walk
// (it was linked to the RAW's item, which the walk deletes)
func TestFormerMainGone(t *testing.T) {
	root := t.TempDir()
	raw, jpeg := filepath.Join(root, "D.NEF"), filepath.Join(root, "D.JPG")
	write(t, raw, "source")
	write(t, jpeg, "derivative")
	scan(t, root)
	rawGuid := itemAt(t, raw).Guid

	if err := os.Remove(raw); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	assertNoItem(t, rawGuid)
	if item := itemAt(t, jpeg); item.State != dto.Visible {
		t.Errorf("the JPEG is not an item after one walk: %+v", item)
	}
}
