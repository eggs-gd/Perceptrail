package folder

import (
	"path/filepath"
	"reflect"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"
)

func entry(path string) dto.ItemEntry {
	return dto.ItemEntry{Path: path, Name: filepath.Base(path)}
}

func names(groups []providers.Group) [][]string {
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
	g := &Grouper{}
	var out []providers.Group
	for _, n := range []string{"IMG_1.HEIC", "IMG_1.MOV", "IMG_1.aae", "a.edited.jpg", "a.jpg", "a.jpg.xmp", "a.xmp", "b", "b.png"} {
		if group, err := g.Decorate(entry("/lib/" + n)); err == nil {
			out = append(out, group)
		}
	}
	last, _ := g.Flush()
	out = append(out, last...)

	want := [][]string{
		{"IMG_1.HEIC", "IMG_1.MOV", "IMG_1.aae"}, // case-insensitive
		{"a.edited.jpg"},
		{"a.jpg", "a.jpg.xmp", "a.xmp"},
		{"b", "b.png"}, // the last group goes out on the walk's flush
	}
	if got := names(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if again, _ := g.Flush(); len(again) != 0 {
		t.Errorf("a second flush gave %v", names(again))
	}

	// Another directory closes the group, even with the same name
	g = &Grouper{}
	g.Decorate(entry("/lib/x.jpg"))
	if group, err := g.Decorate(entry("/lib/sub/x.xmp")); err != nil || len(group.Files) != 1 {
		t.Errorf("a file of another directory joined the group: %v %v", group, err)
	}
}

// .THM posters are not items of their own
func TestSkipPath(t *testing.T) {
	cases := map[string]bool{
		"/Pictures/clip.THM":  true,
		"/Pictures/clip.mov":  false,
		"/Pictures/photo.jpg": false,
	}
	for path, skip := range cases {
		if got := shouldSkipPath(path); got != skip {
			t.Errorf("%s: skip %v, want %v", path, got, skip)
		}
	}
}
