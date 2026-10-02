package routes

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sort"
	"strings"

	"perceptrail/gontroller/pkg/model/dto"

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
	// Apple Photos: better renditions asked for when needed (see rendition.go)
	OnDemand *onDemand `json:"onDemand,omitempty"`
}

// embeddedName: the extracted embedded preview (not a file of the asset)
const embeddedName = "embedded"

func toClientAsset(item *dto.ItemDto, files []*dto.FileDto) clientAsset {
	a := clientAsset{Edit: []rendition{}, Stills: []rendition{}, Motion: []rendition{}, Frames: []rendition{}}
	previewIsFile := false
	for _, f := range files {
		if f.IsIgnored() {
			continue
		}
		r := rendition{URL: fmt.Sprintf("/assets/%s/%d", item.Guid, f.ID), Mime: f.MimeType, W: f.Width, H: f.Height, Codec: f.Codec}
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
		a.Stills = append(a.Stills, rendition{URL: fmt.Sprintf("/assets/%s/%s", item.Guid, embeddedName), Mime: item.PreviewMime, W: w, H: h})
	}
	bySize := func(rs []rendition) {
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].W < rs[j].W })
	}
	bySize(a.Edit)
	bySize(a.Stills)
	a.Kind, a.Duration = assetKind(item, a), item.Duration
	a.OnDemand = onDemandOf(item, a.Original != nil)
	return a
}

// assetKind: what the source said (Apple Photos), else by the roles — a video
// original is a video, a photo with motion is a Live Photo
func assetKind(item *dto.ItemDto, a clientAsset) string {
	switch {
	case item.Kind != "":
		return item.Kind
	case a.Original != nil && strings.HasPrefix(a.Original.Mime, "video/"):
		return dto.KindVideo
	case len(a.Motion) > 0:
		return dto.KindLive
	}
	return dto.KindPhoto
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
