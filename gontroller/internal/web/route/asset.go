package route

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sort"

	"perceptrail/gontroller/internal/model/dto"

	_ "golang.org/x/image/webp"
)

// The asset contract: the client gets every file of an asset once, by role, and
// decides what to show when — a still for the tile (<picture>/srcset: the browser
// picks the size and the format), motion or frames on hover, the biggest still in
// the viewer, the original on demand.

type rendition struct {
	URL   string `json:"url"` // relative to the API: /assets/<guid>/<id>
	Mime  string `json:"mime"`
	W     int    `json:"w,omitempty"`
	H     int    `json:"h,omitempty"`
	Codec string `json:"codec,omitempty"` // video: avc1, hvc1, …
}

type clientAsset struct {
	Kind     string  `json:"kind"`               // photo, live, video: the tile marks moving ones
	Duration float64 `json:"duration,omitempty"` // a video's length, seconds
	// The source; may not be viewable (HEIC, RAW, HEVC). A video original is the
	// asset's motion too.
	Original *rendition  `json:"original"`
	Edit     []rendition `json:"edit"`   // the user's edit, smallest first
	Stills   []rendition `json:"stills"` // viewable images, smallest first
	Motion   []rendition `json:"motion"` // videos (a Live Photo's video)
	Frames   []rendition `json:"frames"` // a flip-book, in order (Apple's video frames)
	// A provider's item (Apple Photos…): better renditions asked for when needed
	// (see rendition.go)
	OnDemand *onDemand `json:"onDemand,omitempty"`
	// The full size of what is seen (oriented; for an edited Photos asset its
	// current version): a rendition this big is the full resolution — the viewer
	// shows it as the Original, the tile's cloud says when none is here
	Full *dims `json:"full,omitempty"`
}

type dims struct {
	W int `json:"w"`
	H int `json:"h"`
}

// embeddedName: the extracted embedded preview (not a file of the asset)
const embeddedName = "embedded"

// renditionMimes: our renditions' formats (not the system's tables: they differ, and
// some have no mp4)
var renditionMimes = map[string]string{"webp": "image/webp", "jpg": "image/jpeg", "mp4": "video/mp4"}

// toClientAsset: every file of the asset by role, for the client; lib: the item's
// library (nil: a plain folder's), which says what can be asked for on demand
func toClientAsset(item *dto.ItemDto, files []*dto.FileDto, lib Library) clientAsset {
	a := clientAsset{Edit: []rendition{}, Stills: []rendition{}, Motion: []rendition{}, Frames: []rendition{}}
	previewIsFile := false
	for _, f := range files {
		if f.IsIgnored() {
			continue
		}
		r := rendition{URL: fmt.Sprintf("/assets/%s/%d", item.GUID, f.ID), Mime: f.MimeType, W: f.Width, H: f.Height, Codec: f.Codec}
		previewIsFile = previewIsFile || f.Path == item.PreviewPath
		switch f.Role {
		case dto.RoleOriginal:
			a.Original = &r
		case dto.RoleEdit:
			a.Edit = append(a.Edit, r)
		case dto.RoleStill:
			a.Stills = append(a.Stills, r)
		case dto.RoleMotion:
			a.Motion = append(a.Motion, r)
		case dto.RoleFrames:
			a.Frames = append(a.Frames, r)
		}
	}
	if item.PreviewPath != "" && !previewIsFile {
		w, h := headerSize(item.PreviewPath)
		a.Stills = append(a.Stills, rendition{URL: fmt.Sprintf("/assets/%s/%s", item.GUID, embeddedName), Mime: item.PreviewMime, W: w, H: h})
	}
	bySize := func(rs []rendition) {
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].W < rs[j].W })
	}
	bySize(a.Edit)
	bySize(a.Stills)
	a.Kind, a.Duration = dto.AssetKind(item.Kind, files), item.Duration
	a.OnDemand = onDemandOf(item, lib)
	if item.Size.W > 0 && item.Size.H > 0 {
		a.Full = &dims{W: int(item.Size.W), H: int(item.Size.H)}
	}
	return a
}

func headerSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// withRenditions: our renditions join the asset — stills (the client's srcset picks
// the one of the width it needs, smallest first) and motion (ours first: H.264 that
// every browser plays, before an original it may not)
func withRenditions(a *clientAsset, item *dto.ItemDto, renditions []dto.RenditionDto) {
	var motion []rendition
	for _, r := range renditions {
		out := rendition{URL: fmt.Sprintf("/assets/%s/r/%s", item.GUID, renditionName(r)),
			Mime: renditionMimes[r.Format], W: r.W, H: r.H}
		if r.Role == dto.RoleMotion {
			out.Codec = "avc1"
			motion = append(motion, out)
			continue
		}
		a.Stills = append(a.Stills, out)
	}
	sort.SliceStable(a.Stills, func(i, j int) bool { return a.Stills[i].W < a.Stills[j].W })
	a.Motion = append(motion, a.Motion...)
	if a.Motion == nil {
		a.Motion = []rendition{} // a list, never null: the client spreads it
	}
}

// renditionName: a rendition's name in its URL
func renditionName(r dto.RenditionDto) string { return fmt.Sprintf("%d.%s", r.Size, r.Format) }
