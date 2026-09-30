package scan

import (
	"perceptrail/gontroller/pkg/model"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/scan/flow"
	"perceptrail/gontroller/pkg/scan/groups"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

func TestMimeRanking(t *testing.T) {
	file := func(name string, size int64) *dto.FileDto {
		return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/" + name, Name: name, Size: size}}
	}
	rank := func(files ...*dto.FileDto) *flow.RawItem {
		g, _ := mimeStep{}.Decorate(&flow.RawItem{Files: files, Exif: make([]api.RawExif, len(files))})
		return g
	}
	cases := []struct {
		files []*dto.FileDto
		main  string
		media bool
	}{
		// The source is the main file; photos next to it are derivatives
		{[]*dto.FileDto{file("IMG_1.HEIC", 2000), file("IMG_1.MOV", 3000)}, "IMG_1.MOV", true}, // Live Photo
		{[]*dto.FileDto{file("D.JPG", 5000), file("D.xmp", 10), file("D.NEF", 30000)}, "D.NEF", true},
		{[]*dto.FileDto{file("P.jpg", 500), file("P.xmp", 10)}, "P.jpg", true},
		{[]*dto.FileDto{file("clip.mp4", 100)}, "clip.mp4", true},
		{[]*dto.FileDto{file("notes.xmp", 10)}, "notes.xmp", false},
	}
	for _, c := range cases {
		g := rank(c.files...)
		if g.Files[0].Name != c.main || g.IsMedia() != c.media {
			t.Errorf("main %s media %v, want %s %v", g.Files[0].Name, g.IsMedia(), c.main, c.media)
		}
	}
	if k, m := kindOf("/x/a.CR2", ""); k != flow.KindRaw || m != "image/x-canon-cr2" {
		t.Errorf("CR2: %s %s", k, m)
	}
	if k, _ := kindOf("/x/a.bin", "image/x-nikon-nef"); k != flow.KindRaw {
		t.Errorf("RAW by exif MIME: %s", k)
	}
}

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

// A move is recognised and needs no work: it does not reach the plugins
func TestMovedSkipsTranscode(t *testing.T) {
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
	if got := scan(t, root); len(got) != 0 {
		t.Errorf("a moved file was processed again: %v", got)
	}
	if item := itemAt(t, moved); item.Guid != guid || item.State != dto.Ready {
		t.Errorf("moved: %+v", item)
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
	gate := newFilesGate(groups.Branches, newProgress(), nil)
	if _, err := gate.Decorate(flow.FileGroup{Files: []*dto.FileDto{{ItemEntry: statEntry(t, notes)}}}); err == nil {
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
	if item := itemAt(t, jpeg); item.State != dto.Ready {
		t.Errorf("the JPEG is not an item after one walk: %+v", item)
	}
}

// A new MIME detection classifies the ignored groups once more
func TestReclassifyIgnored(t *testing.T) {
	root := t.TempDir()
	clip := filepath.Join(root, "clip.mov")
	write(t, clip, "a video the old detection missed")
	f, err := filesProxy.CreateFile(statEntry(t, clip))
	if err != nil {
		t.Fatal(err)
	}
	f.SetIgnored()
	if _, err := filesProxy.UpdateFile(f); err != nil {
		t.Fatal(err)
	}

	meta := model.NewProxy(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	if err := meta.SetMeta(mimeVersionKey, "1"); err != nil {
		t.Fatal(err)
	}
	if err := reclassifyIgnored(meta, filesProxy, logger); err != nil {
		t.Fatal(err)
	}
	if v, _ := meta.GetMeta(mimeVersionKey); v != mimeVersion {
		t.Errorf("version %q, want %q", v, mimeVersion)
	}
	scan(t, root)
	if item := itemAt(t, clip); item.MimeType != "video/quicktime" {
		t.Errorf("the video is still not an item: %+v", item)
	}
}
