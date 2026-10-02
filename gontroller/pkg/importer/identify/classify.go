package identify

import (
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"net/http"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/importer/flow"
	"sort"
	"strings"
)

// mime: what every file of the group is, then which one is the main file. The
// type comes from the content (exiftool), then an own extension table, then a
// content sniff — no system MIME tables (a minimal Docker image has none).

// The main file is always the source: RAW, then video, then an image. The JPEG of
// RAW+JPEG and the photo of a Live Photo are derivatives — later they can serve as
// ready previews and save a transcode, but the item is the source.
var kindRank = map[flow.MediaKind]int{flow.KindRaw: 0, flow.KindVideo: 1, flow.KindImage: 2, flow.KindSidecar: 3, flow.KindOther: 4}

type extInfo struct {
	kind flow.MediaKind
	mime string
}

var extTable = map[string]extInfo{
	".jpg": {flow.KindImage, "image/jpeg"}, ".jpeg": {flow.KindImage, "image/jpeg"},
	".heic": {flow.KindImage, "image/heic"}, ".heif": {flow.KindImage, "image/heif"},
	".png": {flow.KindImage, "image/png"}, ".gif": {flow.KindImage, "image/gif"},
	".webp": {flow.KindImage, "image/webp"}, ".avif": {flow.KindImage, "image/avif"},
	".tif": {flow.KindImage, "image/tiff"}, ".tiff": {flow.KindImage, "image/tiff"},
	".bmp": {flow.KindImage, "image/bmp"},

	".dng": {flow.KindRaw, "image/x-adobe-dng"}, ".cr2": {flow.KindRaw, "image/x-canon-cr2"},
	".cr3": {flow.KindRaw, "image/x-canon-cr3"}, ".nef": {flow.KindRaw, "image/x-nikon-nef"},
	".arw": {flow.KindRaw, "image/x-sony-arw"}, ".raf": {flow.KindRaw, "image/x-fujifilm-raf"},
	".orf": {flow.KindRaw, "image/x-olympus-orf"}, ".rw2": {flow.KindRaw, "image/x-panasonic-rw2"},

	".mov": {flow.KindVideo, "video/quicktime"}, ".mp4": {flow.KindVideo, "video/mp4"},
	".m4v": {flow.KindVideo, "video/x-m4v"}, ".avi": {flow.KindVideo, "video/x-msvideo"},
	".mkv": {flow.KindVideo, "video/x-matroska"}, ".webm": {flow.KindVideo, "video/webm"},
	".mts": {flow.KindVideo, "video/m2ts"}, ".m2ts": {flow.KindVideo, "video/m2ts"},
	".3gp": {flow.KindVideo, "video/3gpp"},

	".xmp": {flow.KindSidecar, "application/rdf+xml"}, ".aae": {flow.KindSidecar, "application/xml"},
}

// Classifier: the classify step's logic
type Classifier struct{}

// Decorate fills Kinds (and every file's MimeType) and puts the main file first
func (Classifier) Decorate(g *flow.RawItem) (*flow.RawItem, error) {
	g.Kinds = make([]flow.MediaKind, len(g.Files))
	for i, f := range g.Files {
		var exifMime string
		if g.Exif[i] != nil {
			exifMime = string(g.Exif[i]["MIMEType"])
		}
		g.Kinds[i], f.MimeType = kindOf(f.Path, exifMime)
	}

	if g.Key != "" {
		return g, nil // the grouper decided the main file
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
	return g, nil
}

func (Classifier) Stop() {}

// setRoles: the main file is the original, what else the group has is by kind — a
// photo is a still (the JPEG of a RAW, the photo of a Live Photo), a video is
// motion, the rest (.xmp, .aae) is metadata
func setRoles(g *flow.RawItem) {
	for i, f := range g.Files {
		switch {
		case i == 0:
			f.Role = dto.RoleOriginal
		case g.Kinds[i] == flow.KindImage || g.Kinds[i] == flow.KindRaw:
			f.Role = dto.RoleStill
		case g.Kinds[i] == flow.KindVideo:
			f.Role = dto.RoleMotion
		default:
			f.Role = dto.RoleMeta
		}
	}
}

// kindOf: the MIME type from the content (exiftool), else the extension table, else
// a sniff of the first bytes; the kind follows from it
func kindOf(path, exifMime string) (flow.MediaKind, string) {
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
	case byExt && known.kind == flow.KindSidecar:
		return flow.KindSidecar, mime
	case strings.HasPrefix(mime, "video/"):
		return flow.KindVideo, mime
	case strings.HasPrefix(mime, "image/"):
		if byExt && known.kind == flow.KindRaw || isRawMime(mime) {
			return flow.KindRaw, mime
		}
		return flow.KindImage, mime
	}
	return flow.KindOther, mime
}

func isRawMime(mime string) bool {
	for _, info := range extTable {
		if info.kind == flow.KindRaw && info.mime == mime {
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
