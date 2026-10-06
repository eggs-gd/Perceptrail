package identify

import (
	"path/filepath"
	"strconv"

	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
)

// Embedded previews, the biggest kind first
var embeddedPreviews = []string{"JpgFromRaw", "PreviewImage", "ThumbnailImage"}

// Browser-viewable images; HEIC/HEIF (Safari only) and RAW are not
var viewableImage = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true,
	"image/webp": true, "image/avif": true, "image/bmp": true,
}

// H.264 plays everywhere; HEVC only in Safari — it waits for the transcode
var viewableVideoCodec = map[string]bool{"avc1": true, "avc3": true}

// preview: what the asset shows right now, without a transcode. Any size counts —
// the expensive stage brings the quality later. In order:
//  1. what the source said to show first (an Apple asset: the edit, the original,
//     then its derivatives);
//  2. the main file itself, if the browser shows it (JPEG, PNG, …; H.264 video);
//  3. the biggest browser-viewable derivative of the group (the JPEG of a RAW);
//  4. the preview embedded in the main file (JpgFromRaw, PreviewImage,
//     ThumbnailImage — the biggest first), extracted by exiftool into
//     <dir>/<guid>/embedded.jpg.
//
// Nothing found: the item waits for the expensive stage (Waiting).
func preview(it *draft, tool Exiftool, dir string, logger *l.Logger) (path, mime string) {
	if path, mime = viewableFile(it); path != "" {
		return path, mime
	}
	for _, tag := range embeddedPreviews {
		if it.Exif[0] == nil || len(it.Exif[0][tag]) == 0 {
			continue
		}
		dst := filepath.Join(dir, it.Item.Guid, "embedded.jpg")
		err := tool.Extract(tag, it.Files[0].Path, dst)
		if err == nil {
			return dst, "image/jpeg"
		}
		logger.Warn("Embedded preview not extracted", l.String("file", it.Files[0].Path), l.String("tag", tag), l.Error(err))
	}
	return "", ""
}

// viewableFile: a file of the group the browser shows (1–3 above), or none
func viewableFile(it *draft) (path, mime string) {
	for _, f := range it.Show {
		if viewableImage[f.MimeType] {
			return f.Path, f.MimeType
		}
	}

	main := it.Files[0]
	if viewable(main, it.Kinds[0], it.Exif[0]) {
		return main.Path, main.MimeType
	}

	best := -1
	for i := 1; i < len(it.Files); i++ {
		if it.Kinds[i] != kindImage || !viewableImage[it.Files[i].MimeType] {
			continue
		}
		if best < 0 || pixels(it.Exif[i], it.Files[i]) > pixels(it.Exif[best], it.Files[best]) {
			best = i
		}
	}
	if best >= 0 {
		return it.Files[best].Path, it.Files[best].MimeType
	}
	return "", ""
}

func viewable(f *dto.FileDto, kind mediaKind, exif map[string][]byte) bool {
	switch kind {
	case kindImage:
		return viewableImage[f.MimeType]
	case kindVideo:
		return exif != nil && viewableVideoCodec[string(exif["CompressorID"])]
	}
	return false
}

// pixels: the image's size for picking the biggest derivative (the file size when
// exif has no dimensions)
func pixels(exif map[string][]byte, f *dto.FileDto) int64 {
	w, _ := strconv.ParseInt(string(exif["ImageWidth"]), 10, 64)
	h, _ := strconv.ParseInt(string(exif["ImageHeight"]), 10, 64)
	if w > 0 && h > 0 {
		return w * h
	}
	return f.Size
}
