package immich

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// An Immich asset is a few files for us, none of them on a disk: the original and
// what Immich made of it, each a path of ours its provider turns into an API call
// (File):
//
//	immich://<id>/original/<name>   the original            /assets/<id>/original
//	immich://<id>/preview.jpg       a still, ~1440 px        /assets/<id>/thumbnail?size=preview
//	immich://<id>/thumbnail.webp    a still, ~250 px         /assets/<id>/thumbnail?size=thumbnail
//	immich://<id>/playback.mp4      a video's playback       /assets/<id>/video/playback
//	immich://<id>/motion/<vid>.mp4  a Live Photo's motion    /assets/<vid>/video/playback
//
// Not obvious:
//   - a Live Photo's video is an asset of its own, hidden from the timeline: its
//     photo's path carries its id;
//   - the playback is H.264 under Immich's default transcode policy ("required");
//   - Immich sizes its stills by the short side (fit "outside"), never enlarged.

// asset: what the search gives of an asset (AssetResponseDto, the fields we read)
type asset struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"` // IMAGE, VIDEO, AUDIO, OTHER
	Checksum         string          `json:"checksum"`
	OriginalFileName string          `json:"originalFileName"`
	OriginalMimeType string          `json:"originalMimeType"`
	LivePhotoVideoID string          `json:"livePhotoVideoId"`
	Width            int             `json:"width"`
	Height           int             `json:"height"`
	Duration         json.RawMessage `json:"duration"` // ms (3.x), "H:MM:SS.ffffff" before
	FileCreatedAt    time.Time       `json:"fileCreatedAt"`
	LocalDateTime    time.Time       `json:"localDateTime"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	IsTrashed        bool            `json:"isTrashed"`
	IsOffline        bool            `json:"isOffline"`
	Exif             *exif           `json:"exifInfo"`
}

// exif: ExifResponseDto, the fields we read
type exif struct {
	DateTimeOriginal *time.Time `json:"dateTimeOriginal"`
	ExifImageWidth   int        `json:"exifImageWidth"`
	ExifImageHeight  int        `json:"exifImageHeight"`
	Orientation      string     `json:"orientation"`
	FileSizeInByte   int64      `json:"fileSizeInByte"`
	Latitude         *float64   `json:"latitude"`
	Longitude        *float64   `json:"longitude"`
	Make             string     `json:"make"`
	Model            string     `json:"model"`
	LensModel        string     `json:"lensModel"`
	ISO              int        `json:"iso"`
	FNumber          float64    `json:"fNumber"`
	ExposureTime     string     `json:"exposureTime"`
	FocalLength      float64    `json:"focalLength"`
	Description      string     `json:"description"`
	Rating           *int       `json:"rating"`
}

// file: one file of an asset as we list it
type file struct {
	path  string
	role  string
	mime  string
	size  int64
	w, h  int
	codec string
}

// The short side of Immich's stills (its defaults)
const (
	previewSide   = 1440
	thumbnailSide = 250
)

// scheme: the prefix of every path of ours
const scheme = "immich://"

// listed: what the library has of an asset — media we show (not audio, not the
// trashed or offline ones)
func (a *asset) listed() bool {
	return a.ID != "" && !a.IsTrashed && !a.IsOffline && (a.Type == "IMAGE" || a.Type == "VIDEO")
}

// files: the asset's files, the original first
func (a *asset) files() []file {
	w, h := a.size()
	dir := scheme + a.ID + "/"
	var size int64
	if a.Exif != nil {
		size = a.Exif.FileSizeInByte
	}
	out := []file{{path: dir + "original/" + a.name(), role: dto.RoleOriginal, mime: a.OriginalMimeType, size: size, w: w, h: h}}
	pw, ph := shortSide(w, h, previewSide)
	tw, th := shortSide(w, h, thumbnailSide)
	out = append(out,
		file{path: dir + "preview.jpg", role: dto.RoleStill, mime: "image/jpeg", w: pw, h: ph},
		file{path: dir + "thumbnail.webp", role: dto.RoleStill, mime: "image/webp", w: tw, h: th})
	if a.Type == "VIDEO" {
		out = append(out, file{path: dir + "playback.mp4", role: dto.RoleMotion, mime: "video/mp4", codec: "avc1"})
	}
	if a.LivePhotoVideoID != "" {
		out = append(out, file{path: dir + "motion/" + a.LivePhotoVideoID + ".mp4", role: dto.RoleMotion, mime: "video/mp4", codec: "avc1"})
	}
	return out
}

// kind: what the asset is (dto.Kind*)
func (a *asset) kind() string {
	switch {
	case a.Type == "VIDEO":
		return dto.KindVideo
	case a.LivePhotoVideoID != "":
		return dto.KindLive
	default:
		return dto.KindPhoto
	}
}

// meta: what Immich knows of the asset, as exiftool -n would give it (no groups),
// so the perceptors read it unchanged: the date it shows (local, its offset), the
// oriented size, the place, the length, the camera
func (a *asset) meta() api.RawExif {
	r := api.RawExif{}
	set := func(tag, v string) {
		if v != "" {
			r[tag] = []byte(v)
		}
	}
	taken := a.FileCreatedAt
	if a.Exif != nil && a.Exif.DateTimeOriginal != nil {
		taken = *a.Exif.DateTimeOriginal
	}
	if !taken.IsZero() {
		// localDateTime is the wall clock written as UTC: its difference from the
		// instant is the zone's offset
		offset := a.LocalDateTime.Sub(taken).Round(time.Minute)
		if !a.LocalDateTime.IsZero() && offset.Abs() <= 14*time.Hour {
			set("DateTimeOriginal", a.LocalDateTime.UTC().Format("2006:01:02 15:04:05"))
			set("OffsetTimeOriginal", formatOffset(offset))
		} else {
			set("GPSDateTime", taken.UTC().Format("2006:01:02 15:04:05")+"Z")
		}
		if ms := taken.Nanosecond() / int(time.Millisecond); ms > 0 {
			set("SubSecTimeOriginal", fmt.Sprintf("%03d", ms))
		}
	}
	if w, h := a.size(); w > 0 && h > 0 {
		set("ImageWidth", strconv.Itoa(w))
		set("ImageHeight", strconv.Itoa(h))
		// The size is oriented: a file's Orientation/Rotation must not turn it again
		set("Orientation", "1")
		set("Rotation", "0")
	}
	if d := a.duration(); d > 0 {
		set("Duration", strconv.FormatFloat(d, 'f', -1, 64))
	}
	if e := a.Exif; e != nil {
		if e.Latitude != nil && e.Longitude != nil {
			set("GPSLatitude", strconv.FormatFloat(*e.Latitude, 'f', -1, 64))
			set("GPSLongitude", strconv.FormatFloat(*e.Longitude, 'f', -1, 64))
		}
		set("Make", e.Make)
		set("Model", e.Model)
		set("LensModel", e.LensModel)
		set("ExposureTime", e.ExposureTime)
		set("ImageDescription", e.Description)
		if e.ISO > 0 {
			set("ISO", strconv.Itoa(e.ISO))
		}
		if e.FNumber > 0 {
			set("FNumber", strconv.FormatFloat(e.FNumber, 'f', -1, 64))
		}
		if e.FocalLength > 0 {
			set("FocalLength", strconv.FormatFloat(e.FocalLength, 'f', -1, 64))
		}
		if e.Rating != nil {
			set("Rating", strconv.Itoa(*e.Rating))
		}
	}
	return r
}

// size: the asset's oriented size — Immich's own, else the EXIF's turned by its
// orientation (5–8: a quarter turn)
func (a *asset) size() (int, int) {
	if a.Width > 0 && a.Height > 0 {
		return a.Width, a.Height
	}
	if a.Exif == nil {
		return 0, 0
	}
	w, h := a.Exif.ExifImageWidth, a.Exif.ExifImageHeight
	if o, _ := strconv.Atoi(a.Exif.Orientation); o >= 5 && o <= 8 {
		w, h = h, w
	}
	return w, h
}

// duration: a video's length in seconds (0: none or unknown)
func (a *asset) duration() float64 {
	raw := strings.TrimSpace(string(a.Duration))
	if raw == "" || raw == "null" {
		return 0
	}
	if ms, err := strconv.ParseFloat(raw, 64); err == nil {
		return ms / 1000
	}
	var clock string // "H:MM:SS.ffffff"
	if json.Unmarshal(a.Duration, &clock) != nil {
		return 0
	}
	parts := strings.Split(clock, ":")
	if len(parts) != 3 {
		return 0
	}
	hours, err1 := strconv.Atoi(parts[0])
	minutes, err2 := strconv.Atoi(parts[1])
	seconds, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}
	return float64(hours*3600+minutes*60) + seconds
}

// name: the original's name, safe as one path segment
func (a *asset) name() string {
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(a.OriginalFileName)
	if name == "" {
		name = "original"
	}
	return name
}

// shortSide: w×h scaled so its short side is side, never enlarged
func shortSide(w, h, side int) (int, int) {
	short := min(w, h)
	if short <= 0 || short <= side {
		return w, h
	}
	return (w*side + short/2) / short, (h*side + short/2) / short
}

func formatOffset(d time.Duration) string {
	sign := '+'
	if d < 0 {
		sign, d = '-', -d
	}
	minutes := int(d / time.Minute)
	return fmt.Sprintf("%c%02d:%02d", sign, minutes/60, minutes%60)
}
