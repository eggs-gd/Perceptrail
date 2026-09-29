package scan

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// mime: what every file of the group is, then which one is the main file. The
// type comes from the content (exiftool), then an own extension table, then a
// content sniff — no system MIME tables (a minimal Docker image has none).

// MediaKind is the role a file can play in a group
type MediaKind string

const (
	KindImage   MediaKind = "image"
	KindRaw     MediaKind = "raw"
	KindVideo   MediaKind = "video"
	KindSidecar MediaKind = "sidecar"
	KindOther   MediaKind = "other"
)

// The main file is always the source: RAW, then video, then an image. The JPEG of
// RAW+JPEG and the photo of a Live Photo are derivatives — later they can serve as
// ready previews and save a transcode, but the item is the source.
var kindRank = map[MediaKind]int{KindRaw: 0, KindVideo: 1, KindImage: 2, KindSidecar: 3, KindOther: 4}

type extInfo struct {
	kind MediaKind
	mime string
}

var extTable = map[string]extInfo{
	".jpg": {KindImage, "image/jpeg"}, ".jpeg": {KindImage, "image/jpeg"},
	".heic": {KindImage, "image/heic"}, ".heif": {KindImage, "image/heif"},
	".png": {KindImage, "image/png"}, ".gif": {KindImage, "image/gif"},
	".webp": {KindImage, "image/webp"}, ".avif": {KindImage, "image/avif"},
	".tif": {KindImage, "image/tiff"}, ".tiff": {KindImage, "image/tiff"},
	".bmp": {KindImage, "image/bmp"},

	".dng": {KindRaw, "image/x-adobe-dng"}, ".cr2": {KindRaw, "image/x-canon-cr2"},
	".cr3": {KindRaw, "image/x-canon-cr3"}, ".nef": {KindRaw, "image/x-nikon-nef"},
	".arw": {KindRaw, "image/x-sony-arw"}, ".raf": {KindRaw, "image/x-fujifilm-raf"},
	".orf": {KindRaw, "image/x-olympus-orf"}, ".rw2": {KindRaw, "image/x-panasonic-rw2"},

	".mov": {KindVideo, "video/quicktime"}, ".mp4": {KindVideo, "video/mp4"},
	".m4v": {KindVideo, "video/x-m4v"}, ".avi": {KindVideo, "video/x-msvideo"},
	".mkv": {KindVideo, "video/x-matroska"}, ".webm": {KindVideo, "video/webm"},
	".mts": {KindVideo, "video/m2ts"}, ".m2ts": {KindVideo, "video/m2ts"},
	".3gp": {KindVideo, "video/3gpp"},

	".xmp": {KindSidecar, "application/rdf+xml"}, ".aae": {KindSidecar, "application/xml"},
}

type mimeStep struct{}

func (mimeStep) Decorate(g exifGroup) (exifGroup, error) {
	g.kinds = make([]MediaKind, len(g.files))
	for i, f := range g.files {
		var exifMime string
		if g.exifs[i] != nil {
			exifMime = string(g.exifs[i]["MIMEType"])
		}
		g.kinds[i], f.MimeType = kindOf(f.Path, exifMime)
	}

	// Rank: the main file first. Ties: the bigger file, then the name — deterministic
	order := make([]int, len(g.files))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if ri, rj := kindRank[g.kinds[i]], kindRank[g.kinds[j]]; ri != rj {
			return ri < rj
		}
		if g.files[i].Size != g.files[j].Size {
			return g.files[i].Size > g.files[j].Size
		}
		return g.files[i].Name < g.files[j].Name
	})
	files, exifs, kinds := g.files[:0:0], g.exifs[:0:0], g.kinds[:0:0]
	for _, i := range order {
		files = append(files, g.files[i])
		exifs = append(exifs, g.exifs[i])
		kinds = append(kinds, g.kinds[i])
	}
	g.files, g.exifs, g.kinds = files, exifs, kinds

	switch g.kinds[0] {
	case KindImage, KindRaw, KindVideo:
		g.media = true
	}
	return g, nil
}

func (mimeStep) Stop() {}

// kindOf: the MIME type from the content (exiftool), else the extension table, else
// a sniff of the first bytes; the kind follows from it
func kindOf(path, exifMime string) (MediaKind, string) {
	ext := strings.ToLower(filepath.Ext(path))
	known, byExt := extTable[ext]

	mime := exifMime
	if mime == "" && byExt {
		mime = known.mime
	}
	if mime == "" {
		mime = sniff(path)
	}

	switch {
	case byExt && known.kind == KindSidecar:
		return KindSidecar, mime
	case strings.HasPrefix(mime, "video/"):
		return KindVideo, mime
	case strings.HasPrefix(mime, "image/"):
		if byExt && known.kind == KindRaw || isRawMime(mime) {
			return KindRaw, mime
		}
		return KindImage, mime
	}
	return KindOther, mime
}

func isRawMime(mime string) bool {
	for _, info := range extTable {
		if info.kind == KindRaw && info.mime == mime {
			return true
		}
	}
	return false
}

func sniff(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return ""
	}
	return http.DetectContentType(buf[:n])
}
