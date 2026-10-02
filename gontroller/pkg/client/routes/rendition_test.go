package routes

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"

	"github.com/labstack/echo/v4"
)

// fakePhotos stands in for Photos: asked for a rendition, it writes the file where
// Photos would (the naming layout) — or fails
type fakePhotos struct {
	root  string
	asked []string
	fail  bool
}

func (f *fakePhotos) put(uuid, name string) error {
	f.asked = append(f.asked, name)
	if f.fail {
		return errors.New("offline")
	}
	p := filepath.Join(f.root, "resources", "derivatives", uuid[:1], uuid+name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(name), 0o644)
}

func (f *fakePhotos) Image(uuid string, size int) error { return f.put(uuid, "_1_102_o.jpeg") }
func (f *fakePhotos) Video(uuid string, mode int) error {
	if mode == videoFast {
		return f.put(uuid, "_2_4_o.mp4")
	}
	return f.put(uuid, "_2_201_o.mov")
}
func (f *fakePhotos) Live(uuid string) error { return f.put(uuid, "_2_101_o.mov") }

// On demand: a rendition not on disk is asked for once, then served from the
// library; what is local is served without asking; nothing to ask for: 404
func TestRenditionOnDemand(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Photos Library.photoslibrary")
	photos := &fakePhotos{root: root}
	e := echo.New()
	RegisterRenditionRoutes(e, photos, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))

	put := func(guid, kind, path string) {
		t.Helper()
		if _, err := itemsProxy.UpdateItem(&dto.ItemDto{Guid: guid, State: dto.Visible, Kind: kind, Path: path}); err != nil {
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

	get := func(path string) (int, string) {
		t.Helper()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
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
		code, body := get(tc.path)
		if code != tc.code || (tc.body != "" && body != tc.body) || len(photos.asked) != tc.asked {
			t.Errorf("%s: %d %q, asked %v; want %d %q, asked %d times", tc.path, code, body, photos.asked, tc.code, tc.body, tc.asked)
		}
	}

	// Photos fails (offline, no access) and nothing is local: 404, the client keeps
	// what it shows
	photos.fail = true
	put("E5555555-OFFLINE", dto.KindPhoto, thumb("E5555555-OFFLINE"))
	if code, _ := get("/items/E5555555-OFFLINE/rendition/medium"); code != 404 {
		t.Errorf("offline: %d, want 404", code)
	}
}

// The client learns which items can ask for more: those from Photos, a hover for
// what moves
func TestOnDemandInAsset(t *testing.T) {
	lib := "/p/Photos Library.photoslibrary/originals/A/A1.heic"
	if od := toClientAsset(&dto.ItemDto{Guid: "A1", Kind: dto.KindPhoto, Path: lib}, nil).OnDemand; od == nil ||
		od.Medium != "/items/A1/rendition/medium" || od.Hover != "" {
		t.Errorf("photo: %+v", od)
	}
	if od := toClientAsset(&dto.ItemDto{Guid: "V1", Kind: dto.KindVideo, Path: lib}, nil).OnDemand; od == nil ||
		od.Hover != "/items/V1/rendition/hover" {
		t.Errorf("video: %+v", od)
	}
	if od := toClientAsset(&dto.ItemDto{Guid: "F1", Path: "/photos/f.jpg"}, nil).OnDemand; od != nil {
		t.Errorf("a folder's photo: %+v, want none", od)
	}
}
