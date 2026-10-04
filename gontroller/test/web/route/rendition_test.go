package route_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"perceptrail/gontroller/internal/library/apple"
	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/test/fake"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// The Apple library through the HTTP API: its renditions with a fake Photos; the
// dispatch by library

// On demand: a rendition not on disk is asked for once, then served from the
// library; what is local is served without asking; nothing to ask for: 404
func TestRenditionOnDemand(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Photos Library.photoslibrary")
	photos := &fake.Photos{Root: root}
	lib := apple.New(filepath.Dir(root), photos, testDB, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))
	e := server(func(item *dto.ItemDto) provider.Renditions {
		if lib.Owns(item) {
			return lib
		}
		return nil
	})

	put := func(guid, kind, path string) {
		t.Helper()
		if _, err := testDB.UpdateItem(&dto.ItemDto{Guid: guid, State: dto.Visible, Kind: kind, Path: path}); err != nil {
			t.Fatal(err)
		}
	}
	thumb := func(uuid string) string {
		return filepath.Join(root, "resources", "derivatives", "masters", uuid[:1], uuid+"_4_5005_c.jpeg")
	}
	put("A1111111-PHOTO", dto.KindPhoto, thumb("A1111111-PHOTO"))
	put("B2222222-VIDEO", dto.KindVideo, thumb("B2222222-VIDEO"))
	put("C3333333-LIVE", dto.KindLive, thumb("C3333333-LIVE"))
	put("D4444444-FOLDER", dto.KindPhoto, "/photos/d.jpg")

	fetch := func(path string) (int, string) {
		rec := get(e, path)
		return rec.Code, rec.Body.String()
	}

	for _, tc := range []struct {
		path, body string
		code       int
		asked      int // how many requests to Photos so far
	}{
		{"/items/A1111111-PHOTO/rendition/medium", "_1_102_o.jpeg", 200, 1},
		{"/items/A1111111-PHOTO/rendition/medium", "_1_102_o.jpeg", 200, 1}, // local now: not asked again
		{"/items/A1111111-PHOTO/rendition/hover", "", 404, 1},               // a photo does not move
		{"/items/B2222222-VIDEO/rendition/medium?hevc=0", "_2_4_o.mp4", 200, 2},
		{"/items/B2222222-VIDEO/rendition/hover", "_2_4_o.mp4", 200, 2}, // the 360p is the hover
		{"/items/B2222222-VIDEO/rendition/medium", "_2_201_o.mov", 200, 3},
		{"/items/C3333333-LIVE/rendition/hover", "_2_101_o.mov", 200, 4},
		{"/items/D4444444-FOLDER/rendition/medium", "", 404, 4}, // not from Photos
		{"/items/NOPE/rendition/medium", "", 404, 4},
	} {
		code, body := fetch(tc.path)
		if code != tc.code || (tc.body != "" && body != tc.body) || len(photos.Asked) != tc.asked {
			t.Errorf("%s: %d %q, asked %v; want %d %q, asked %d times", tc.path, code, body, photos.Asked, tc.code, tc.body, tc.asked)
		}
	}

	// The original: a photo's current version at full resolution as JPEG, a video's
	// file
	if code, body := fetch("/items/A1111111-PHOTO/rendition/original"); code != 200 || body != "FULL JPEG" {
		t.Errorf("original: %d %q, want the full JPEG", code, body)
	}
	if code, body := fetch("/items/B2222222-VIDEO/rendition/original"); code != 200 || body != "_ORIGINAL.mov" {
		t.Errorf("video original: %d %q", code, body)
	}

	// Drawn from a local original (a HEIC): no file — the JPEG Photos handed over
	photos.Draw = true
	put("F6666666-HEIC", dto.KindPhoto, filepath.Join(root, "originals/F/F6666666-HEIC.heic"))
	if code, body := fetch("/items/F6666666-HEIC/rendition/medium"); code != 200 || body != "JPEG" {
		t.Errorf("drawn: %d %q, want the JPEG", code, body)
	}
	photos.Draw = false

	// Photos fails (offline, no access) and nothing is local: 404, the client keeps
	// what it shows
	photos.Fail = true
	put("E5555555-OFFLINE", dto.KindPhoto, thumb("E5555555-OFFLINE"))
	if code, _ := fetch("/items/E5555555-OFFLINE/rendition/medium"); code != 404 {
		t.Errorf("offline: %d, want 404", code)
	}
}

// The info panel's files: every file of the group, sidecars too, each downloadable
func TestItemFiles(t *testing.T) {
	e := server(nil)
	if _, err := testDB.UpdateItem(&dto.ItemDto{Guid: "FILES-1", State: dto.Visible}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, role string }{{"a.heic", dto.RoleOriginal}, {"a.xmp", dto.RoleMeta}} {
		file, err := testDB.CreateFile(dto.ItemEntry{Path: "/lib/" + f.name, Name: f.name, Size: 10})
		if err != nil {
			t.Fatal(err)
		}
		file.LinkedTo, file.Role = "FILES-1", f.role
		if _, err := testDB.UpdateFile(file); err != nil {
			t.Fatal(err)
		}
	}
	rec := get(e, "/items/FILES-1/files")
	var files []struct{ Role, URL string }
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[1].Role != dto.RoleMeta || !strings.HasPrefix(files[1].URL, "/assets/FILES-1/") {
		t.Errorf("files %+v, want the original and its sidecar, downloadable", files)
	}

}
