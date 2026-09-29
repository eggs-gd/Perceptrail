package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

func entry(path string) dto.ItemEntry {
	return dto.ItemEntry{Path: path, Name: filepath.Base(path)}
}

func names(groups []FileGroup) [][]string {
	var out [][]string
	for _, g := range groups {
		var n []string
		for _, e := range g.Files {
			n = append(n, e.Name)
		}
		out = append(out, n)
	}
	return out
}

// Files come in name order; one group is open, the next name closes it
func TestGenericGrouper(t *testing.T) {
	g := newGenericGrouper()
	var out []FileGroup
	for _, n := range []string{"IMG_1.HEIC", "IMG_1.MOV", "IMG_1.aae", "a.edited.jpg", "a.jpg", "a.jpg.xmp", "a.xmp", "b", "b.png"} {
		if group, err := g.Decorate(fileEvent{entry: entry("/lib/" + n)}); err == nil {
			out = append(out, group)
		}
	}
	marker := &walkResult{}
	last, _ := g.Decorate(fileEvent{done: marker})
	out = append(out, last)

	want := [][]string{
		{"IMG_1.HEIC", "IMG_1.MOV", "IMG_1.aae"}, // case-insensitive
		{"a.edited.jpg"},
		{"a.jpg", "a.jpg.xmp", "a.xmp"},
		{"b", "b.png"}, // the last group goes out with the marker
	}
	if got := names(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if last.Done != marker {
		t.Error("the marker must come with the last group")
	}

	// Another directory closes the group, even with the same name
	g = newGenericGrouper()
	g.Decorate(fileEvent{entry: entry("/lib/x.jpg")})
	if group, err := g.Decorate(fileEvent{entry: entry("/lib/sub/x.xmp")}); err != nil || len(group.Files) != 1 {
		t.Errorf("a file of another directory joined the group: %v %v", group, err)
	}
}

func TestSourceSwitch(t *testing.T) {
	lib := fileEvent{entry: entry("/Pictures/Photos Library.photoslibrary/originals/A/x.heic")}
	got, _ := sourceSwitch{}.Switch(lib)
	if _, ok := got[branchGeneric]; !ok || len(got) != 1 {
		t.Errorf("with the Apple Photos grouper off the library goes to generic, got %v", got)
	}
	marker, _ := sourceSwitch{}.Switch(fileEvent{done: &walkResult{}})
	if len(marker) != groupBranches {
		t.Errorf("the marker must reach every grouper, got %d", len(marker))
	}
	if !inPhotosLibrary(lib.entry.Path) || inPhotosLibrary("/Pictures/a.jpg") {
		t.Error("inPhotosLibrary")
	}
}

func TestMimeRanking(t *testing.T) {
	file := func(name string, size int64) *dto.FileDto {
		return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/" + name, Name: name, Size: size}}
	}
	rank := func(files ...*dto.FileDto) *RawItem {
		g, _ := mimeStep{}.Decorate(&RawItem{Files: files, Exif: make([]api.RawExif, len(files))})
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
		if g.Files[0].Name != c.main || g.isMedia() != c.media {
			t.Errorf("main %s media %v, want %s %v", g.Files[0].Name, g.isMedia(), c.main, c.media)
		}
	}
	if k, m := kindOf("/x/a.CR2", ""); k != KindRaw || m != "image/x-canon-cr2" {
		t.Errorf("CR2: %s %s", k, m)
	}
	if k, _ := kindOf("/x/a.bin", "image/x-nikon-nef"); k != KindRaw {
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

func TestTranscodeSwitch(t *testing.T) {
	route := func(kinds ...MediaKind) int {
		out, _ := transcodeSwitch{}.Switch(&RawItem{Kinds: kinds})
		for b := range out {
			return b
		}
		return -1
	}
	if route(KindVideo, KindImage) != branchLivePhoto || route(KindVideo) != branchVideo ||
		route(KindRaw, KindImage) != branchPhoto || route(KindImage, KindSidecar) != branchPhoto {
		t.Error("wrong transcode branch")
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
	gate := newFilesGate(groupBranches, nil)
	if _, err := gate.Decorate(FileGroup{Files: []*dto.FileDto{{ItemEntry: statEntry(t, notes)}}}); err == nil {
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
