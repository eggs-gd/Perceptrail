// An Immich library through the server's own start and the import's whole chain,
// against a fake Immich: its assets become items without a file on our disk, the
// routes serve their files from it, and an asset is gone only after a complete
// listing that does not have it
package immich_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/importer"
	"perceptrail/gontroller/internal/library"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/perceptor"
	"perceptrail/gontroller/internal/web/route"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
	"github.com/eggs-gd/perceplib/api"
	"github.com/labstack/echo/v4"
)

const (
	photoID  = "11111111-1111-4111-8111-111111111111"
	liveID   = "22222222-2222-4222-8222-222222222222"
	motionID = "33333333-3333-4333-8333-333333333333"
	videoID  = "44444444-4444-4444-8444-444444444444"
)

var assets = map[string]string{
	photoID: `{"id":"` + photoID + `","type":"IMAGE","checksum":"c1","originalFileName":"IMG_1.HEIC","originalMimeType":"image/heic",
	  "width":3024,"height":4032,"fileCreatedAt":"2024-07-01T10:00:00Z","localDateTime":"2024-07-01T13:00:00Z",
	  "updatedAt":"2024-07-02T00:00:00Z","exifInfo":{"dateTimeOriginal":"2024-07-01T10:00:00Z","latitude":50.45,"longitude":30.52}}`,
	liveID: `{"id":"` + liveID + `","type":"IMAGE","checksum":"c2","originalFileName":"IMG_2.HEIC","originalMimeType":"image/heic",
	  "livePhotoVideoId":"` + motionID + `","width":4032,"height":3024,"fileCreatedAt":"2024-07-01T11:00:00Z",
	  "localDateTime":"2024-07-01T14:00:00Z","updatedAt":"2024-07-02T00:00:00Z"}`,
	videoID: `{"id":"` + videoID + `","type":"VIDEO","checksum":"c3","originalFileName":"MVI_3.MOV","originalMimeType":"video/quicktime",
	  "width":1920,"height":1080,"duration":12500,"fileCreatedAt":"2024-07-03T08:00:00Z","localDateTime":"2024-07-03T08:00:00Z",
	  "updatedAt":"2024-07-04T00:00:00Z"}`,
}

var (
	testDB *model.Proxy
	immich = &fakeImmich{listed: []string{photoID, liveID, videoID}}
)

// fakeImmich: the timeline it lists now, an asset a page; refuses: it fails after
// the first page. Its files answer their API path.
type fakeImmich struct {
	mu      sync.Mutex
	listed  []string
	refuses bool
}

func TestMain(m *testing.M) {
	server := httptest.NewServer(immich)
	dir, err := os.MkdirTemp("", "gontroller-immich-test")
	if err != nil {
		panic(err)
	}
	// The server's own start, with an Immich: its url in the config, its key in the env
	os.Setenv("IMMICH_API_KEY", "test-key")
	cfgFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(cfgFile, []byte("providers:\n  immich:\n    url: "+server.URL+"\n"), 0o644); err != nil {
		panic(err)
	}
	cfg, err := config.Read(cfgFile)
	if err != nil {
		panic(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	if testDB, err = model.Open(cfg, logger); err != nil {
		panic(err)
	}
	if err := perceptor.Load(cfg, logger); err != nil {
		panic(err)
	}
	if err := library.Enable(cfg, testDB, logger); err != nil {
		panic(err)
	}
	code := m.Run()
	server.Close()
	os.RemoveAll(dir)
	os.Exit(code)
}

// The timeline imported: an item per asset, keyed by its id, shown at once by
// Immich's preview (no render of ours), its date and place Immich's; the routes
// serve its files from Immich. Then: a refused listing deletes nothing, a complete
// one without an asset deletes its item.
func TestImmichLibrary(t *testing.T) {
	scan(t)
	for id, kind := range map[string]string{photoID: dto.KindPhoto, liveID: dto.KindLive, videoID: dto.KindVideo} {
		item, err := testDB.GetItemByGUID(api.GUID(id))
		if err != nil {
			t.Fatalf("no item for %s: %v", id, err)
		}
		if item.State != dto.Visible || item.Kind != kind || item.PreviewPath != "immich://"+id+"/preview.jpg" {
			t.Errorf("%s: state %v, kind %q, preview %q", id, item.State, item.Kind, item.PreviewPath)
		}
	}
	photo, _ := testDB.GetItemByGUID(photoID)
	if got := photo.GetDate().Format(time.RFC3339); got != "2024-07-01T13:00:00+03:00" {
		t.Errorf("photo's date %s", got)
	}

	e := echo.New()
	route.Register(e, testDB, func(item *dto.ItemDto) route.Library {
		if lib := library.Of(item); lib != nil {
			return lib
		}
		return nil
	}, t.TempDir(), route.AppInfo{}, nil, nil, l.NewLogger(l.FatalLevel, &tree.Decorator{}))
	if body := get(t, e, "/assets/"+photoID); body != "/api/assets/"+photoID+"/thumbnail?size=preview" {
		t.Errorf("the item's preview: %q", body)
	}
	var files []struct{ Name, URL string }
	json.Unmarshal([]byte(get(t, e, "/items/"+liveID+"/files")), &files)
	served := map[string]string{}
	for _, f := range files {
		served[f.Name] = get(t, e, f.URL)
	}
	if served["IMG_2.HEIC"] != "/api/assets/"+liveID+"/original" || served[motionID+".mp4"] != "/api/assets/"+motionID+"/video/playback" {
		t.Errorf("the Live Photo's files: %v", served)
	}

	immich.set(true, photoID, liveID) // the photo listed, then a failure
	scan(t)
	if _, err := testDB.GetItemByGUID(videoID); err != nil {
		t.Error("a listing broken off deleted the video")
	}
	immich.set(false, photoID, liveID)
	scan(t)
	if _, err := testDB.GetItemByGUID(videoID); err == nil {
		t.Error("the video gone from Immich is still an item")
	}
	if _, err := testDB.GetItemByGUID(photoID); err != nil {
		t.Error("the photo went with it")
	}
}

func (f *fakeImmich) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("x-api-key") != "test-key" {
		http.Error(w, "no key", http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/api/search/metadata" {
		io.WriteString(w, r.URL.RequestURI())
		return
	}
	var req struct{ Cursor string }
	json.NewDecoder(r.Body).Decode(&req)
	page, _ := strconv.Atoi(req.Cursor)
	if f.refuses && page > 0 {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	next := "null"
	if page+1 < len(f.listed) {
		next = strconv.Quote(strconv.Itoa(page + 1))
	}
	fmt.Fprintf(w, `{"assets":{"items":[%s],"nextCursor":%s}}`, assets[f.listed[page]], next)
}

func (f *fakeImmich) set(refuses bool, listed ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuses, f.listed = refuses, listed
}

// scan: one pass of the import, no roots on a disk — Immich only
func scan(t *testing.T) {
	t.Helper()
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	if err := importer.New(pass{cache: t.TempDir()}, testDB, logger).Pass(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, e *echo.Echo, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET %s: %d", path, rec.Code)
	}
	return rec.Body.String()
}

// pass: what the import reads of the config
type pass struct{ cache string }

func (p pass) LibraryRoots() []string { return nil }
func (p pass) CacheDir() string       { return p.cache }
func (p pass) Rescan() time.Duration  { return time.Minute }
func (p pass) Exiftool() string       { return "exiftool" }
