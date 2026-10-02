package routes

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/scan/groups/apple"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

// On demand (Apple Photos): the renditions Photos keeps only in iCloud are asked for
// when the client needs them — never in bulk. Photos downloads them into its own
// library; this serves the file, and the next walk adds it to the asset (the delta
// brings it to the client). Roadmap step 0, findings "PhotoKit spike".
//
//	GET /items/:guid/rendition/medium  the viewer: the image (~2048 px, or the edit),
//	                                   a video's 720p (?hevc=0: H.264 only)
//	GET /items/:guid/rendition/hover   a tile's hover: a video's 360p, a Live Photo's
//	                                   motion
//	GET /items/:guid/rendition/original  the original: a photo's (a Live Photo's
//	                                   photo's) at full resolution as JPEG, unedited —
//	                                   any browser shows it; ?file=1 the file itself
//	                                   (HEIC, RAW: a download); a video's original file
//
// What is on disk is served without asking; nothing local and nothing to ask (not
// macOS, no access, not a Photos item): 404, the client keeps what it shows. An
// image Photos draws from a local original (a HEIC) leaves no file: the JPEG it
// handed over is served (not kept — the browser caches it).

// Fetcher asks the source for a rendition it does not keep locally (photokit.Library).
// Image returns the image as JPEG too (nil if none); Full the unedited original at
// full resolution as JPEG; Original the original file with its type (UTI) and name;
// Video the file it made local.
type Fetcher interface {
	Image(uuid string, size int) ([]byte, error)
	Full(uuid string) ([]byte, error)
	Original(uuid string) ([]byte, string, string, error)
	Video(uuid string, mode int) (string, error)
	Live(uuid string) error
}

// Video delivery modes (photokit): the original only when asked for
const (
	videoOriginal = 1
	videoMedium   = 2
	videoFast     = 3
)

// The viewer's image: Photos' ~2048 px rendition
const mediumSize = 2048

// At most this many requests to Photos at once (a viewer opening, its neighbours,
// hovers)
const fetchers = 3

var (
	fetcher Fetcher
	// changed: Photos made something local — the walk goes sooner (scan.WalkSoon)
	changed  = func() {}
	fetchSem = make(chan struct{}, fetchers)
	// One request per asset and want at a time: the others wait for its result
	inFlightMu sync.Mutex
	inFlight   = map[string]*fetching{}
)

type fetching struct {
	done chan struct{}
	data []byte
	err  error
}

// RegisterRenditionRoutes: f may be nil (only what is on disk is served);
// libraryChanged is called when Photos made a file local (nil: nothing)
func RegisterRenditionRoutes(e *echo.Echo, f Fetcher, libraryChanged func(), logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	fetcher = f
	if libraryChanged != nil {
		changed = libraryChanged
	}
	e.GET("/items/:guid/rendition/:level", func(c echo.Context) error { return getRendition(c, logger) })
}

func getRendition(c echo.Context, logger *l.Logger) error {
	item, err := itemsProxy.GetItemByGuid(c.Param("guid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	root := apple.BundleRoot(item.Path)
	if root == "" {
		return echo.NewHTTPError(http.StatusNotFound) // not from Photos: nothing to ask for
	}
	if c.Param("level") == "original" {
		return getOriginal(c, item, logger)
	}
	want, ask, ok := wantOf(item.Kind, c.Param("level"), c.QueryParam("hevc") != "0")
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	uuid := item.Guid // an Apple item's GUID is its asset UUID
	path := apple.Local(root, uuid, want)
	var drawn []byte
	if path == "" && fetcher != nil {
		var err error
		drawn, err = once(uuid+"/"+c.Param("level"), func() ([]byte, error) { return ask(fetcher, uuid) })
		if err != nil {
			drawn = nil // a failed request hands over nothing to show
			logger.Debug("Rendition not fetched", l.String("guid", uuid), l.String("level", c.Param("level")), l.Error(err))
		} else {
			changed()
		}
		path = apple.Local(root, uuid, want)
	}
	c.Response().Header().Set("Cache-Control", "private, max-age=86400")
	if path == "" && len(drawn) > 0 {
		return c.Blob(http.StatusOK, "image/jpeg", drawn) // drawn from a local original: no file
	}
	if path == "" && want == apple.WantImage && viewableOriginal(item.Path) {
		path = item.Path // a local JPEG/PNG original: Photos had nothing smaller to make
	}
	if path == "" {
		c.Response().Header().Del("Cache-Control")
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return c.File(path)
}

type askFunc func(Fetcher, string) ([]byte, error)

func noData(_ string, err error) ([]byte, error) { return nil, err }

// getOriginal: asked for by the user (the viewer's Original), one at a time per
// request — no sharing between callers, the semaphore still holds
func getOriginal(c echo.Context, item *dto.ItemDto, logger *l.Logger) error {
	if fetcher == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	fetchSem <- struct{}{}
	defer func() { <-fetchSem }()
	fail := func(err error) error {
		logger.Debug("Original not fetched", l.String("guid", item.Guid), l.Error(err))
		return echo.NewHTTPError(http.StatusNotFound)
	}
	// Photos keeps the original local now: the item learns of it with the next walk
	defer changed()
	c.Response().Header().Set("Cache-Control", "private, max-age=86400")
	switch {
	case item.Kind == dto.KindVideo:
		path, err := fetcher.Video(item.Guid, videoOriginal)
		if err != nil || path == "" {
			return fail(err)
		}
		return c.File(path)
	case c.QueryParam("file") == "1":
		data, uti, name, err := fetcher.Original(item.Guid)
		if err != nil {
			return fail(err)
		}
		if name != "" {
			c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
		}
		return c.Blob(http.StatusOK, mimeOfUTI(uti), data)
	default:
		data, err := fetcher.Full(item.Guid)
		if err != nil {
			return fail(err)
		}
		return c.Blob(http.StatusOK, "image/jpeg", data)
	}
}

// mimeOfUTI: the original's type for the browser (a download names it right)
func mimeOfUTI(uti string) string {
	switch uti {
	case "public.jpeg":
		return "image/jpeg"
	case "public.png":
		return "image/png"
	case "public.heic":
		return "image/heic"
	case "public.heif":
		return "image/heif"
	case "public.tiff":
		return "image/tiff"
	case "com.compuserve.gif":
		return "image/gif"
	}
	return "application/octet-stream" // RAW and the rest
}

// wantOf: what a level means for the item's kind, and how to ask Photos for it
func wantOf(kind, level string, hevc bool) (apple.Want, askFunc, bool) {
	switch {
	case level == "medium" && kind == dto.KindVideo && hevc:
		return apple.WantVideo, func(f Fetcher, u string) ([]byte, error) { return noData(f.Video(u, videoMedium)) }, true
	case level == "medium" && kind == dto.KindVideo:
		// No H.264 720p for an iPhone video from Photos: its 360p
		return apple.WantVideoH264, func(f Fetcher, u string) ([]byte, error) { return noData(f.Video(u, videoFast)) }, true
	case level == "medium":
		return apple.WantImage, func(f Fetcher, u string) ([]byte, error) { return f.Image(u, mediumSize) }, true
	case level == "hover" && kind == dto.KindVideo:
		return apple.WantVideoHover, func(f Fetcher, u string) ([]byte, error) { return noData(f.Video(u, videoFast)) }, true
	case level == "hover" && kind == dto.KindLive:
		return apple.WantLiveMotion, func(f Fetcher, u string) ([]byte, error) { return nil, f.Live(u) }, true
	}
	return 0, nil, false
}

// once runs fetch for key unless one runs already (then waits for its result), at
// most `fetchers` at a time
func once(key string, fetch func() ([]byte, error)) ([]byte, error) {
	inFlightMu.Lock()
	if f, ok := inFlight[key]; ok {
		inFlightMu.Unlock()
		<-f.done
		return f.data, nil // the first caller logs the error
	}
	f := &fetching{done: make(chan struct{})}
	inFlight[key] = f
	inFlightMu.Unlock()
	defer func() {
		inFlightMu.Lock()
		delete(inFlight, key)
		inFlightMu.Unlock()
		close(f.done)
	}()
	fetchSem <- struct{}{}
	defer func() { <-fetchSem }()
	f.data, f.err = fetch()
	return f.data, f.err
}

func viewableOriginal(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}

// HydrateWaiting: an asset from Photos with nothing viewable on disk (Optimize Mac
// Storage purged even its thumbnail — 7 of 6 427 in the dev library) is Waiting:
// never on the sheet, so never opened, so never asked for. Those are asked for
// here, in the background, each once per run; when Photos has downloaded the image
// the next walk finds it and the item shows. Every `every` until ctx ends.
func HydrateWaiting(ctx context.Context, every time.Duration, logger *l.Logger) {
	if fetcher == nil {
		return
	}
	asked := map[string]bool{}
	for {
		hydrateRound(asked, logger)
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func hydrateRound(asked map[string]bool, logger *l.Logger) {
	items, err := itemsProxy.GetItemsInStates(dto.Waiting)
	if err != nil {
		logger.Error("Waiting items not read", l.Error(err))
		return
	}
	for _, it := range items {
		if asked[it.Guid] || apple.BundleRoot(it.Path) == "" {
			continue
		}
		asked[it.Guid] = true
		// The image even for a video: its poster is what the tile shows
		if _, err := once(it.Guid+"/medium", func() ([]byte, error) { return fetcher.Image(it.Guid, mediumSize) }); err != nil {
			logger.Debug("Waiting asset not fetched", l.String("guid", it.Guid), l.Error(err))
		} else {
			changed()
		}
	}
}

// onDemandOf: the client's on-demand renditions of an item from Photos (nil for
// others): the viewer's medium, a hover for what moves, the original when it is not
// here (a Live Photo's original is its video: its photo's is always asked for)
func onDemandOf(item *dto.ItemDto, originalHere bool) *onDemand {
	if apple.BundleRoot(item.Path) == "" {
		return nil
	}
	base := "/items/" + item.Guid + "/rendition/"
	od := &onDemand{Medium: base + "medium"}
	if item.Kind == dto.KindVideo || item.Kind == dto.KindLive {
		od.Hover = base + "hover"
	}
	if !originalHere || item.Kind == dto.KindLive {
		od.Original = base + "original"
	}
	return od
}

type onDemand struct {
	Medium   string `json:"medium"`             // relative to the API
	Hover    string `json:"hover,omitempty"`    // a video or a Live Photo
	Original string `json:"original,omitempty"` // + "?file=1": the file itself (a photo)
}
