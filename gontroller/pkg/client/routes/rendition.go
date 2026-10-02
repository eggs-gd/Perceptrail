package routes

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"

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
//
// What is on disk is served without asking; nothing local and nothing to ask (not
// macOS, no access, not a Photos item): 404, the client keeps what it shows.

// Fetcher asks the source for a rendition it does not keep locally (photokit.Library)
type Fetcher interface {
	Image(uuid string, size int) error
	Video(uuid string, mode int) error
	Live(uuid string) error
}

// Video delivery modes (photokit): never the ones that download the original
const (
	videoMedium = 2
	videoFast   = 3
)

// The viewer's image: Photos' ~2048 px rendition
const mediumSize = 2048

// At most this many requests to Photos at once (a viewer opening, its neighbours,
// hovers)
const fetchers = 3

var (
	fetcher  Fetcher
	fetchSem = make(chan struct{}, fetchers)
	// One request per asset and want at a time: the others wait for it
	inFlightMu sync.Mutex
	inFlight   = map[string]chan struct{}{}
)

// RegisterRenditionRoutes: f may be nil (only what is on disk is served)
func RegisterRenditionRoutes(e *echo.Echo, f Fetcher, logger *l.Logger) {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger)
	}
	fetcher = f
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
	want, ask, ok := wantOf(item.Kind, c.Param("level"), c.QueryParam("hevc") != "0")
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	uuid := item.Guid // an Apple item's GUID is its asset UUID
	path := apple.Local(root, uuid, want)
	if path == "" && fetcher != nil {
		if err := once(uuid+"/"+c.Param("level"), func() error { return ask(fetcher, uuid) }); err != nil {
			logger.Debug("Rendition not fetched", l.String("guid", uuid), l.String("level", c.Param("level")), l.Error(err))
		}
		path = apple.Local(root, uuid, want)
	}
	if path == "" && want == apple.WantImage && viewableOriginal(item.Path) {
		path = item.Path // a local JPEG/PNG original: Photos had nothing smaller to make
	}
	if path == "" {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	c.Response().Header().Set("Cache-Control", "private, max-age=86400")
	return c.File(path)
}

// wantOf: what a level means for the item's kind, and how to ask Photos for it
func wantOf(kind, level string, hevc bool) (apple.Want, func(Fetcher, string) error, bool) {
	switch {
	case level == "medium" && kind == dto.KindVideo && hevc:
		return apple.WantVideo, func(f Fetcher, u string) error { return f.Video(u, videoMedium) }, true
	case level == "medium" && kind == dto.KindVideo:
		// No H.264 720p for an iPhone video from Photos: its 360p
		return apple.WantVideoH264, func(f Fetcher, u string) error { return f.Video(u, videoFast) }, true
	case level == "medium":
		return apple.WantImage, func(f Fetcher, u string) error { return f.Image(u, mediumSize) }, true
	case level == "hover" && kind == dto.KindVideo:
		return apple.WantVideoHover, func(f Fetcher, u string) error { return f.Video(u, videoFast) }, true
	case level == "hover" && kind == dto.KindLive:
		return apple.WantLiveMotion, func(f Fetcher, u string) error { return f.Live(u) }, true
	}
	return 0, nil, false
}

// once runs fetch for key unless one runs already (then waits for it), at most
// `fetchers` at a time
func once(key string, fetch func() error) error {
	inFlightMu.Lock()
	if ch, ok := inFlight[key]; ok {
		inFlightMu.Unlock()
		<-ch
		return nil // the first caller logs its error; the caller looks at the disk again
	}
	ch := make(chan struct{})
	inFlight[key] = ch
	inFlightMu.Unlock()
	defer func() {
		inFlightMu.Lock()
		delete(inFlight, key)
		inFlightMu.Unlock()
		close(ch)
	}()
	fetchSem <- struct{}{}
	defer func() { <-fetchSem }()
	return fetch()
}

func viewableOriginal(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}

// onDemandOf: the client's on-demand renditions of an item from Photos (nil for
// others): the viewer's medium, and a hover for what moves
func onDemandOf(item *dto.ItemDto) *onDemand {
	if apple.BundleRoot(item.Path) == "" {
		return nil
	}
	base := "/items/" + item.Guid + "/rendition/"
	od := &onDemand{Medium: base + "medium"}
	if item.Kind == dto.KindVideo || item.Kind == dto.KindLive {
		od.Hover = base + "hover"
	}
	return od
}

type onDemand struct {
	Medium string `json:"medium"`          // relative to the API
	Hover  string `json:"hover,omitempty"` // a video or a Live Photo
}
