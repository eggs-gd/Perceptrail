package identify

import (
	"testing"

	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

func TestMimeRanking(t *testing.T) {
	file := func(name string, size int64) *dto.FileDto {
		return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/" + name, Name: name, Size: size}}
	}
	rank := func(files ...*dto.FileDto) *flow.RawItem {
		g, _ := Classifier{}.Decorate(&flow.RawItem{Files: files, Exif: make([]api.RawExif, len(files))})
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

// Roles of a generic group: the source is the original, a photo next to it a
// still, a video next to a photo motion, .xmp metadata
func TestGenericRoles(t *testing.T) {
	file := func(name string, size int64) *dto.FileDto {
		return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/" + name, Name: name, Size: size}}
	}
	g, _ := Classifier{}.Decorate(&flow.RawItem{
		Files: []*dto.FileDto{file("D.JPG", 5000), file("D.xmp", 10), file("D.NEF", 30000), file("D.MOV", 900)},
		Exif:  make([]api.RawExif, 4),
	})
	want := map[string]string{"D.NEF": dto.RoleOriginal, "D.JPG": dto.RoleStill, "D.xmp": dto.RoleMeta}
	for _, f := range g.Files {
		if w, ok := want[f.Name]; ok && f.Role != w {
			t.Errorf("%s: role %q, want %q", f.Name, f.Role, w)
		}
	}
	lp, _ := Classifier{}.Decorate(&flow.RawItem{
		Files: []*dto.FileDto{file("L.HEIC", 2000), file("L.MOV", 3000)},
		Exif:  make([]api.RawExif, 2),
	})
	if lp.Files[0].Name != "L.MOV" || lp.Files[0].Role != dto.RoleOriginal || lp.Files[1].Role != dto.RoleStill {
		t.Errorf("Live Photo: %s %s / %s %s", lp.Files[0].Name, lp.Files[0].Role, lp.Files[1].Name, lp.Files[1].Role)
	}
}
