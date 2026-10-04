package identify

import (
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// mime: what every file of the group is, then which one is the main file. The
// type comes from the content (exiftool), then an own extension table, then a
// content sniff — no system MIME tables (a minimal Docker image has none).

// The main file is always the source: RAW, then video, then an image. The JPEG of
// RAW+JPEG and the photo of a Live Photo are derivatives — later they can serve as
// ready previews and save a transcode, but the item is the source.
var kindRank = map[mediaKind]int{kindRaw: 0, kindVideo: 1, kindImage: 2, kindSidecar: 3, kindOther: 4}

type extInfo struct {
	kind mediaKind
	mime string
}

var extTable = map[string]extInfo{
	".jpg": {kindImage, "image/jpeg"}, ".jpeg": {kindImage, "image/jpeg"},
	".heic": {kindImage, "image/heic"}, ".heif": {kindImage, "image/heif"},
	".png": {kindImage, "image/png"}, ".gif": {kindImage, "image/gif"},
	".webp": {kindImage, "image/webp"}, ".avif": {kindImage, "image/avif"},
	".tif": {kindImage, "image/tiff"}, ".tiff": {kindImage, "image/tiff"},
	".bmp": {kindImage, "image/bmp"},

	".dng": {kindRaw, "image/x-adobe-dng"}, ".cr2": {kindRaw, "image/x-canon-cr2"},
	".cr3": {kindRaw, "image/x-canon-cr3"}, ".nef": {kindRaw, "image/x-nikon-nef"},
	".arw": {kindRaw, "image/x-sony-arw"}, ".raf": {kindRaw, "image/x-fujifilm-raf"},
	".orf": {kindRaw, "image/x-olympus-orf"}, ".rw2": {kindRaw, "image/x-panasonic-rw2"},

	".mov": {kindVideo, "video/quicktime"}, ".mp4": {kindVideo, "video/mp4"},
	".m4v": {kindVideo, "video/x-m4v"}, ".avi": {kindVideo, "video/x-msvideo"},
	".mkv": {kindVideo, "video/x-matroska"}, ".webm": {kindVideo, "video/webm"},
	".mts": {kindVideo, "video/m2ts"}, ".m2ts": {kindVideo, "video/m2ts"},
	".3gp": {kindVideo, "video/3gpp"},

	".xmp": {kindSidecar, "application/rdf+xml"}, ".aae": {kindSidecar, "application/xml"},
}

// classify fills Kinds (and every file's MimeType) and puts the main file first
func classify(g *draft) {
	g.Kinds = make([]mediaKind, len(g.Files))
	for i, f := range g.Files {
		var exifMime string
		if g.Exif[i] != nil {
			exifMime = string(g.Exif[i]["MIMEType"])
		}
		g.Kinds[i], f.MimeType = kindOf(f.Path, exifMime)
	}

	if g.Key != "" {
		return // the grouper decided the main file
	}

	// Rank: the main file first. Ties: the bigger file, then the name — deterministic
	order := make([]int, len(g.Files))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if ri, rj := kindRank[g.Kinds[i]], kindRank[g.Kinds[j]]; ri != rj {
			return ri < rj
		}
		if g.Files[i].Size != g.Files[j].Size {
			return g.Files[i].Size > g.Files[j].Size
		}
		return g.Files[i].Name < g.Files[j].Name
	})
	files, exifs, kinds := g.Files[:0:0], g.Exif[:0:0], g.Kinds[:0:0]
	for _, i := range order {
		files = append(files, g.Files[i])
		exifs = append(exifs, g.Exif[i])
		kinds = append(kinds, g.Kinds[i])
	}
	g.Files, g.Exif, g.Kinds = files, exifs, kinds
	setRoles(g)
}

// setRoles: the main file is the original, what else the group has is by kind — a
// photo is a still (the JPEG of a RAW, the photo of a Live Photo), a video is
// motion, the rest (.xmp, .aae) is metadata
func setRoles(g *draft) {
	for i, f := range g.Files {
		switch {
		case i == 0:
			f.Role = dto.RoleOriginal
		case g.Kinds[i] == kindImage || g.Kinds[i] == kindRaw:
			f.Role = dto.RoleStill
		case g.Kinds[i] == kindVideo:
			f.Role = dto.RoleMotion
		default:
			f.Role = dto.RoleMeta
		}
	}
}

// kindOf: the MIME type from the content (exiftool), else the extension table, else
// a sniff of the first bytes; the kind follows from it
func kindOf(path, exifMime string) (mediaKind, string) {
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
	case byExt && known.kind == kindSidecar:
		return kindSidecar, mime
	case strings.HasPrefix(mime, "video/"):
		return kindVideo, mime
	case strings.HasPrefix(mime, "image/"):
		if byExt && known.kind == kindRaw || isRawMime(mime) {
			return kindRaw, mime
		}
		return kindImage, mime
	}
	return kindOther, mime
}

func isRawMime(mime string) bool {
	for _, info := range extTable {
		if info.kind == kindRaw && info.mime == mime {
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

// mimeVersion changes whenever the kind detection changes. Groups ignored by an
// older detection are classified once more (the system MIME tables lost HEIC,
// MOV, RAW in a minimal Docker image: those groups were ignored for good).
const mimeVersion = "2"

const mimeVersionKey = "mime_version"

// KindsStore: what the kinds' table version needs — the meta table (the version
// the files were judged by) and the "ignored" marks it clears
type KindsStore interface {
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
	UnignoreFiles() (int64, error)
}

// reclassifyIgnored runs at start: a new mimeVersion clears every "ignored" mark,
// the gate then sends those groups through mime again
func reclassifyIgnored(db KindsStore, logger *l.Logger) error {
	stored, err := db.GetMeta(mimeVersionKey)
	if err != nil || stored == mimeVersion {
		return err
	}
	n, err := db.UnignoreFiles()
	if err != nil {
		return err
	}
	logger.Info("MIME detection changed: ignored files are classified again",
		l.String("from", stored), l.String("to", mimeVersion), l.Int("files", int(n)))
	return db.SetMeta(mimeVersionKey, mimeVersion)
}
