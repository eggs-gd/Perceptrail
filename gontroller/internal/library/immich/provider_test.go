package immich

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"perceptrail/gontroller/internal/model/dto"

	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
)

const (
	testKey   = "test-key"
	photoID   = "11111111-1111-4111-8111-111111111111"
	liveID    = "22222222-2222-4222-8222-222222222222"
	motionID  = "33333333-3333-4333-8333-333333333333"
	videoID   = "44444444-4444-4444-8444-444444444444"
	trashedID = "55555555-5555-4555-8555-555555555555"
)

// The timeline in two pages: a photo taken in Kyiv in summer (+03:00), a Live Photo
// and a trashed one; then a video (its length as 3.x gives it: ms)
var pages = []string{
	`{"assets":{"items":[
	  {"id":"` + photoID + `","type":"IMAGE","checksum":"c1","originalFileName":"IMG_1.HEIC","originalMimeType":"image/heic",
	   "width":3024,"height":4032,"duration":null,"fileCreatedAt":"2024-07-01T10:00:00.250Z","localDateTime":"2024-07-01T13:00:00.250Z",
	   "updatedAt":"2024-07-02T00:00:00Z","exifInfo":{"dateTimeOriginal":"2024-07-01T10:00:00.250Z","fileSizeInByte":2000000,
	   "latitude":50.45,"longitude":30.52,"make":"Apple","model":"iPhone 15","iso":64,"fNumber":1.8,"rating":4}},
	  {"id":"` + liveID + `","type":"IMAGE","checksum":"c2","originalFileName":"IMG_2.HEIC","originalMimeType":"image/heic",
	   "livePhotoVideoId":"` + motionID + `","width":0,"height":0,"fileCreatedAt":"2024-07-01T11:00:00Z","localDateTime":"2024-07-01T14:00:00Z",
	   "updatedAt":"2024-07-02T00:00:00Z","exifInfo":{"exifImageWidth":4032,"exifImageHeight":3024,"orientation":"6"}},
	  {"id":"` + trashedID + `","type":"IMAGE","isTrashed":true,"originalFileName":"gone.jpg","updatedAt":"2024-07-02T00:00:00Z"}
	],"nextCursor":"page2"}}`,
	`{"assets":{"items":[
	  {"id":"` + videoID + `","type":"VIDEO","checksum":"c3","originalFileName":"MVI_3.MOV","originalMimeType":"video/quicktime",
	   "width":1920,"height":1080,"duration":12500,"fileCreatedAt":"2024-07-03T08:00:00Z","localDateTime":"2024-07-03T08:00:00Z",
	   "updatedAt":"2024-07-04T00:00:00Z"}
	],"nextCursor":null}}`,
}

var testLogger = l.NewLogger(l.ErrorLevel, &tree.Decorator{})

// fakeImmich: the search (by cursor) and the files, each request checked for the
// key; files answer their own path and query, and the Range they were asked for
func fakeImmich(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != testKey {
			http.Error(w, "no key", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Cookie") != "" {
			t.Errorf("the browser's cookie reached Immich: %s", r.URL)
		}
		if r.URL.Path == "/api/search/metadata" {
			var req searchRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || r.Method != http.MethodPost {
				t.Errorf("search: %s %v", r.Method, err)
			}
			if !req.WithExif || req.Filter.Visibility.Eq != "timeline" {
				t.Errorf("search asked %+v", req)
			}
			page := 0
			if req.Cursor == "page2" {
				page = 1
			}
			io.WriteString(w, pages[page])
			return
		}
		w.Header().Set("Set-Cookie", "immich=1")
		io.WriteString(w, r.URL.RequestURI()+" "+r.Header.Get("Range"))
	}))
}

func newTestProvider(t *testing.T, server string) *Provider {
	t.Helper()
	p, err := New(server, testKey, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// list: what the listing emits, as paths
func list(t *testing.T, p *Provider) []dto.ItemEntry {
	t.Helper()
	var entries []dto.ItemEntry
	if err := p.List(t.Context(), func(e dto.ItemEntry) bool { entries = append(entries, e); return true }); err != nil {
		t.Fatal(err)
	}
	return entries
}

// The listing: every page, an asset's files together, the trashed one left out
func TestList(t *testing.T) {
	server := fakeImmich(t)
	defer server.Close()
	var paths []string
	for _, e := range list(t, newTestProvider(t, server.URL)) {
		paths = append(paths, e.Path)
	}
	want := []string{
		"immich://" + photoID + "/original/IMG_1.HEIC", "immich://" + photoID + "/preview.jpg", "immich://" + photoID + "/thumbnail.webp",
		"immich://" + liveID + "/original/IMG_2.HEIC", "immich://" + liveID + "/preview.jpg", "immich://" + liveID + "/thumbnail.webp",
		"immich://" + liveID + "/motion/" + motionID + ".mp4",
		"immich://" + videoID + "/original/MVI_3.MOV", "immich://" + videoID + "/preview.jpg", "immich://" + videoID + "/thumbnail.webp",
		"immich://" + videoID + "/playback.mp4",
	}
	if !slices.Equal(paths, want) {
		t.Errorf("listed\n%s\nwant\n%s", strings.Join(paths, "\n"), strings.Join(want, "\n"))
	}
}

// A server that refuses (a wrong key): an error — the walk deletes nothing of it
func TestListRefused(t *testing.T) {
	server := fakeImmich(t)
	defer server.Close()
	p, _ := New(server.URL, "wrong", testLogger)
	err := p.List(t.Context(), func(dto.ItemEntry) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("err %v", err)
	}
}

// Each asset one group, as its files arrive: keyed by its id, the original first,
// Immich's metadata and checksum, its stills to show
func TestGroup(t *testing.T) {
	server := fakeImmich(t)
	defer server.Close()
	p := newTestProvider(t, server.URL)
	var groups []dto.Asset
	for i, e := range list(t, p) {
		g, err := p.Grouper().Decorate(dto.WalkedFile{FileDto: &dto.FileDto{ID: uint(i + 1), ItemEntry: e}})
		if err == chain.ErrSkippedItem {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		groups = append(groups, g)
	}
	if len(groups) != 3 {
		t.Fatalf("%d groups", len(groups))
	}
	photo, live, video := groups[0], groups[1], groups[2]
	if photo.Key != photoID || photo.Fingerprint != "immich:c1" || photo.Kind != dto.KindPhoto || photo.Files[0].Role != dto.RoleOriginal {
		t.Errorf("photo %+v", photo)
	}
	meta := map[string]string{"DateTimeOriginal": "2024:07:01 13:00:00", "OffsetTimeOriginal": "+03:00", "SubSecTimeOriginal": "250",
		"ImageWidth": "3024", "ImageHeight": "4032", "GPSLatitude": "50.45", "Make": "Apple", "ISO": "64", "FNumber": "1.8", "Rating": "4"}
	for tag, want := range meta {
		if got := string(photo.Meta[tag]); got != want {
			t.Errorf("photo %s = %q, want %q", tag, got, want)
		}
	}
	if len(photo.Show) != 2 || photo.Show[0].Name != "preview.jpg" || photo.Show[0].Width != 1440 || photo.Show[0].Height != 1920 {
		t.Errorf("photo shows %+v", photo.Show[0])
	}
	// No size of Immich's own: the EXIF's, turned by its orientation
	if live.Kind != dto.KindLive || string(live.Meta["ImageWidth"]) != "3024" || live.Files[3].Role != dto.RoleMotion || live.Files[3].Codec != "avc1" {
		t.Errorf("live %+v, meta %v", live, live.Meta)
	}
	if video.Kind != dto.KindVideo || string(video.Meta["Duration"]) != "12.5" || video.Files[3].Name != "playback.mp4" {
		t.Errorf("video %+v, meta %v", video, video.Meta)
	}
	if g, err := p.Grouper().Decorate(dto.WalkedFile{FileDto: &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "immich://" + trashedID + "/preview.jpg"}}, Missing: true}); err != nil || len(g.Missing) != 1 {
		t.Errorf("a missing file: %+v, %v", g, err)
	}
}

// A file served through the provider: the API's URL with the key, the Range passed
// on, the browser's cookie not; Immich's cookie not passed back
func TestFile(t *testing.T) {
	server := fakeImmich(t)
	defer server.Close()
	p := newTestProvider(t, server.URL)
	for path, want := range map[string]string{
		"immich://" + photoID + "/original/IMG_1.HEIC":        "/api/assets/" + photoID + "/original",
		"immich://" + photoID + "/preview.jpg":                "/api/assets/" + photoID + "/thumbnail?size=preview",
		"immich://" + photoID + "/thumbnail.webp":             "/api/assets/" + photoID + "/thumbnail?size=thumbnail",
		"immich://" + videoID + "/playback.mp4":               "/api/assets/" + videoID + "/video/playback",
		"immich://" + liveID + "/motion/" + motionID + ".mp4": "/api/assets/" + motionID + "/video/playback",
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/assets/x/1", nil)
		req.Header.Set("Range", "bytes=0-9")
		req.Header.Set("Cookie", "session=browser")
		rec := httptest.NewRecorder()
		p.File(path).ServeHTTP(rec, req)
		if got := rec.Body.String(); got != want+" bytes=0-9" || rec.Header().Get("Set-Cookie") != "" {
			t.Errorf("%s: %q (cookie %q), want %q", path, got, rec.Header().Get("Set-Cookie"), want)
		}
	}
	for _, path := range []string{"/lib/a.jpg", "immich://" + photoID + "/other.jpg", "immich://../preview.jpg",
		"immich://1111111/-1111-4111-8111-111111111111/preview.jpg", "immich://zzzzzzzz-1111-4111-8111-111111111111/preview.jpg",
		"immich://" + liveID + "/motion/..%2fx.mp4"} {
		if p.File(path) != nil {
			t.Errorf("%s served", path)
		}
	}
}

// The key: the env first, else the file's first line; none is an error
func TestKey(t *testing.T) {
	file := filepath.Join(t.TempDir(), "immich.key")
	os.WriteFile(file, []byte("  from-file \nrest\n"), 0o600)
	t.Setenv("IMMICH_API_KEY", "")
	if key, err := Key(file); err != nil || key != "from-file" {
		t.Errorf("file: %q, %v", key, err)
	}
	if _, err := Key(""); err == nil {
		t.Error("no key accepted")
	}
	t.Setenv("IMMICH_API_KEY", "from-env")
	if key, _ := Key(file); key != "from-env" {
		t.Errorf("env: %q", key)
	}
}

// Before 3.x the length was a clock
func TestDurationClock(t *testing.T) {
	a := asset{Duration: json.RawMessage(`"0:01:02.500000"`)}
	if d := a.duration(); d != 62.5 {
		t.Errorf("duration %v", d)
	}
}
