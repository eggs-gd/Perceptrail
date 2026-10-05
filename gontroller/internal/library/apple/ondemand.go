package apple

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
)

// On demand: the renditions Photos keeps only in iCloud are asked for when the
// client needs them — never in bulk. Photos downloads them into its own library;
// the web service serves the file, and the asset's item is marked for the next
// pass (the delta brings it to the client). See the library README, "Apple Photos".
//
//	medium    the viewer: the image (~2048 px, or the edit); a video's 720p (H.264
//	          only when the browser plays no HEVC)
//	hover     a tile's hover: a video's 360p, a Live Photo's motion
//	original  the biggest of what the user sees: a photo's (a Live Photo's photo's)
//	          current version — the edit, cropped — at full resolution as JPEG, any
//	          browser shows it; a video's original file
//
// What is on disk is served without asking; nothing local and nothing to ask (not
// macOS, no access): no rendition, the client keeps what it shows. An image Photos
// draws from a local original (a HEIC) leaves no file: the JPEG it handed over is
// served (not kept — the browser caches it).

// Photos asks Photos for a rendition it does not keep locally (photokit.Library).
// Image returns the image as JPEG too (nil if none); Full the current version (the
// edit) at full resolution as JPEG; Video the file it made local.
type Photos interface {
	Authorize() bool
	Image(uuid string, size int) ([]byte, error)
	Full(uuid string) ([]byte, error)
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

type fetching struct {
	done chan struct{}
	data []byte
	err  error
}

// Levels: the viewer's medium and the Original — always: the biggest of what the
// user sees is Photos' current version (the edit), not a local unedited original
// (edits and their history are Photos' business, not ours); a hover for what moves
func (p *Provider) Levels(item *dto.ItemDto) []string {
	if item.Kind == dto.KindVideo || item.Kind == dto.KindLive {
		return []string{"medium", "hover", "original"}
	}
	return []string{"medium", "original"}
}

func (p *Provider) Rendition(item *dto.ItemDto, level string, opt provider.Options) (provider.Rendition, error) {
	root := BundleRoot(item.Path)
	if root == "" {
		return provider.Rendition{}, provider.ErrNoRendition
	}
	if level == "original" {
		return p.original(item)
	}
	want, ask, ok := wantOf(item.Kind, level, opt.HEVC)
	if !ok {
		return provider.Rendition{}, provider.ErrNoRendition
	}
	uuid := item.Guid // an Apple item's GUID is its asset UUID
	path := Local(root, uuid, want)
	var drawn []byte
	if path == "" {
		var err error
		drawn, err = p.once(uuid+"/"+level, func() ([]byte, error) { return ask(p.photos, uuid) })
		if err != nil {
			drawn = nil // a failed request hands over nothing to show
			p.logger.Debug("Rendition not fetched", l.String("guid", uuid), l.String("level", level), l.Error(err))
		} else {
			// The new file reaches the item on the next walk (the client has its own
			// guess meanwhile: the cloud goes when it got the rendition)
			p.refresh(uuid)
		}
		path = Local(root, uuid, want)
	}
	switch {
	case path != "":
		return provider.Rendition{Path: path}, nil
	case len(drawn) > 0:
		return provider.Rendition{Data: drawn, Mime: "image/jpeg"}, nil // drawn from a local original: no file
	case want == WantImage && viewableOriginal(item.Path):
		return provider.Rendition{Path: item.Path}, nil // a local JPEG/PNG original: nothing smaller to make
	}
	return provider.Rendition{}, provider.ErrNoRendition
}

// original: asked for by the user (the viewer's Original), one at a time per
// request — no sharing between callers, the limit still holds
func (p *Provider) original(item *dto.ItemDto) (provider.Rendition, error) {
	p.sem <- struct{}{}
	defer func() { <-p.sem }()
	fail := func(err error) (provider.Rendition, error) {
		p.logger.Debug("Original not fetched", l.String("guid", item.Guid), l.Error(err))
		return provider.Rendition{}, provider.ErrNoRendition
	}
	if item.Kind == dto.KindVideo {
		path, err := p.photos.Video(item.Guid, videoOriginal)
		if err != nil || path == "" {
			return fail(err)
		}
		p.refresh(item.Guid) // the original is local now: the next walk knows it
		return provider.Rendition{Path: path}, nil
	}
	data, err := p.photos.Full(item.Guid)
	if err != nil {
		return fail(err)
	}
	// Drawing it may have made Photos download the original: the next walk knows it
	p.refresh(item.Guid)
	return provider.Rendition{Data: data, Mime: "image/jpeg"}, nil
}

type askFunc func(Photos, string) ([]byte, error)

func noData(_ string, err error) ([]byte, error) { return nil, err }

// wantOf: what a level means for the item's kind, and how to ask Photos for it
func wantOf(kind, level string, hevc bool) (Want, askFunc, bool) {
	switch {
	case level == "medium" && kind == dto.KindVideo && hevc:
		return WantVideo, func(f Photos, u string) ([]byte, error) { return noData(f.Video(u, videoMedium)) }, true
	case level == "medium" && kind == dto.KindVideo:
		// No H.264 720p for an iPhone video from Photos: its 360p
		return WantVideoH264, func(f Photos, u string) ([]byte, error) { return noData(f.Video(u, videoFast)) }, true
	case level == "medium":
		return WantImage, func(f Photos, u string) ([]byte, error) { return f.Image(u, mediumSize) }, true
	case level == "hover" && kind == dto.KindVideo:
		return WantVideoHover, func(f Photos, u string) ([]byte, error) { return noData(f.Video(u, videoFast)) }, true
	case level == "hover" && kind == dto.KindLive:
		return WantLiveMotion, func(f Photos, u string) ([]byte, error) { return nil, f.Live(u) }, true
	}
	return 0, nil, false
}

// once runs fetch for key unless one runs already (then waits for its result), at
// most `fetchers` at a time
func (p *Provider) once(key string, fetch func() ([]byte, error)) ([]byte, error) {
	p.inFlightMu.Lock()
	if f, ok := p.inFlight[key]; ok {
		p.inFlightMu.Unlock()
		<-f.done
		return f.data, nil // the first caller logs the error
	}
	f := &fetching{done: make(chan struct{})}
	p.inFlight[key] = f
	p.inFlightMu.Unlock()
	defer func() {
		p.inFlightMu.Lock()
		delete(p.inFlight, key)
		p.inFlightMu.Unlock()
		close(f.done)
	}()
	p.sem <- struct{}{}
	defer func() { <-p.sem }()
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

// hydrateWaiting: an asset from Photos with nothing viewable on disk (Optimize Mac
// Storage purged even its thumbnail — 7 of 6 427 in the dev library) is Waiting:
// never on the sheet, so never opened, so never asked for. Those are asked for
// here, in the background, each once per run; when Photos has downloaded the image
// its item is processed again and shows. Every minute until ctx ends.
func (p *Provider) hydrateWaiting(ctx context.Context) {
	asked := map[string]bool{}
	for {
		p.hydrateRound(asked)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

func (p *Provider) hydrateRound(asked map[string]bool) {
	items, err := p.items.Unshown()
	if err != nil {
		p.logger.Error("Waiting items not read", l.Error(err))
		return
	}
	for _, it := range items {
		if asked[it.Guid] || !p.Owns(it) {
			continue
		}
		asked[it.Guid] = true
		// The image even for a video: its poster is what the tile shows
		if _, err := p.once(it.Guid+"/medium", func() ([]byte, error) { return p.photos.Image(it.Guid, mediumSize) }); err != nil {
			p.logger.Debug("Waiting asset not fetched", l.String("guid", it.Guid), l.Error(err))
		} else {
			p.refresh(it.Guid)
		}
	}
}
