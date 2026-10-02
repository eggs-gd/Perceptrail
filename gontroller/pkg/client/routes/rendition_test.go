package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	draw  bool // draw from a local original: no file, only the JPEG
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

func (f *fakePhotos) Image(uuid string, size int) ([]byte, error) {
	if f.draw {
		f.asked = append(f.asked, "drawn")
		return []byte("JPEG"), nil
	}
	return []byte("JPEG"), f.put(uuid, "_1_102_o.jpeg")
}
func (f *fakePhotos) Video(uuid string, mode int) (string, error) {
	name := "_2_201_o.mov"
	switch mode {
	case videoFast:
		name = "_2_4_o.mp4"
	case videoOriginal:
		name = "_ORIGINAL.mov"
	}
	return filepath.Join(f.root, "resources", "derivatives", uuid[:1], uuid+name), f.put(uuid, name)
}
func (f *fakePhotos) Full(uuid string) ([]byte, error) {
	f.asked = append(f.asked, "full")
	return []byte("FULL JPEG"), nil
}

func (f *fakePhotos) Live(uuid string) error { return f.put(uuid, "_2_101_o.mov") }

// On demand: a rendition not on disk is asked for once, then served from the
// library; what is local is served without asking; nothing to ask for: 404
func TestRenditionOnDemand(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Photos Library.photoslibrary")
	photos := &fakePhotos{root: root}
	e := echo.New()
	RegisterRenditionRoutes(e, photos, nil, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))

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

	// The original: a photo's current version at full resolution as JPEG, a video's
	// file
	if code, body := get("/items/A1111111-PHOTO/rendition/original"); code != 200 || body != "FULL JPEG" {
		t.Errorf("original: %d %q, want the full JPEG", code, body)
	}
	if code, body := get("/items/B2222222-VIDEO/rendition/original"); code != 200 || body != "_ORIGINAL.mov" {
		t.Errorf("video original: %d %q", code, body)
	}

	// Drawn from a local original (a HEIC): no file — the JPEG Photos handed over
	photos.draw = true
	put("F6666666-HEIC", dto.KindPhoto, filepath.Join(root, "originals/F/F6666666-HEIC.heic"))
	if code, body := get("/items/F6666666-HEIC/rendition/medium"); code != 200 || body != "JPEG" {
		t.Errorf("drawn: %d %q, want the JPEG", code, body)
	}
	photos.draw = false

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
		od.Medium != "/items/A1/rendition/medium?v="+contractVersion || od.Hover != "" || od.Original != "/items/A1/rendition/original?v="+contractVersion {
		t.Errorf("photo, original in iCloud: %+v", od)
	}
	here := []*dto.FileDto{{ID: 1, Role: dto.RoleOriginal, LinkedTo: "A2"}}
	here[0].MimeType = "image/heic"
	if od := toClientAsset(&dto.ItemDto{Guid: "A2", Kind: dto.KindPhoto, Path: lib}, here).OnDemand; od == nil || od.Original == "" {
		t.Errorf("photo, original here: %+v, want the original still asked from Photos (it may be edited)", od)
	}
	if od := toClientAsset(&dto.ItemDto{Guid: "V1", Kind: dto.KindVideo, Path: lib}, nil).OnDemand; od == nil ||
		od.Hover != "/items/V1/rendition/hover?v="+contractVersion {
		t.Errorf("video: %+v", od)
	}
	if od := toClientAsset(&dto.ItemDto{Guid: "F1", Path: "/photos/f.jpg"}, nil).OnDemand; od != nil {
		t.Errorf("a folder's photo: %+v, want none", od)
	}
}

// Nothing local at all (Waiting): never on the sheet, so asked for in the
// background — once per run, only Photos assets
func TestHydrateWaiting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Photos Library.photoslibrary")
	photos := &fakePhotos{root: root}
	var refreshed []string
	RegisterRenditionRoutes(echo.New(), photos, func(uuid string, _ time.Duration) bool {
		refreshed = append(refreshed, uuid)
		return true
	}, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))
	for _, it := range []*dto.ItemDto{
		{Guid: "H1111111-WAITING", State: dto.Waiting, Kind: dto.KindPhoto, Path: filepath.Join(root, "originals/H/H1111111-WAITING.heic")},
		{Guid: "H2222222-SHOWN", State: dto.Visible, Kind: dto.KindPhoto, Path: filepath.Join(root, "originals/H/H2222222-SHOWN.heic")},
		{Guid: "H3333333-FOLDER", State: dto.Waiting, Kind: dto.KindPhoto, Path: "/photos/h3.heic"},
	} {
		if _, err := itemsProxy.UpdateItem(it); err != nil {
			t.Fatal(err)
		}
	}
	asked := map[string]bool{}
	logger := l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{})
	hydrateRound(asked, logger)
	hydrateRound(asked, logger) // the next round: not again
	if len(refreshed) != 1 || refreshed[0] != "H1111111-WAITING" {
		t.Errorf("refreshed %v, want the asset Photos made local, once", refreshed)
	}
	if len(photos.asked) != 1 || !asked["H1111111-WAITING"] {
		t.Errorf("asked %v (%v), want the waiting Photos asset once", photos.asked, asked)
	}
	if _, err := os.Stat(filepath.Join(root, "resources/derivatives/H/H1111111-WAITING_1_102_o.jpeg")); err != nil {
		t.Error("the image is not in the library")
	}
}

// The info panel's files: every file of the group, sidecars too, each downloadable;
// the asset carries the full size of what is seen
func TestItemFilesAndFull(t *testing.T) {
	e := echo.New()
	RegisterAssetsRoutes("/assets", e, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))
	if _, err := itemsProxy.UpdateItem(&dto.ItemDto{Guid: "FILES-1", State: dto.Visible}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, role string }{{"a.heic", dto.RoleOriginal}, {"a.xmp", dto.RoleMeta}} {
		file, err := filesProxy.CreateFile(dto.ItemEntry{Path: "/lib/" + f.name, Name: f.name, Size: 10})
		if err != nil {
			t.Fatal(err)
		}
		file.LinkedTo, file.Role = "FILES-1", f.role
		if _, err := filesProxy.UpdateFile(file); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/FILES-1/files", nil))
	var files []itemFile
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[1].Role != dto.RoleMeta || !strings.HasPrefix(files[1].URL, "/assets/FILES-1/") {
		t.Errorf("files %+v, want the original and its sidecar, downloadable", files)
	}

	item := &dto.ItemDto{Guid: "FULL-1"}
	item.Size.W, item.Size.H = 3024, 4032
	if a := toClientAsset(item, nil); a.Full == nil || a.Full.W != 3024 || a.Full.H != 4032 {
		t.Errorf("full %+v, want the item's size", a.Full)
	}
}
