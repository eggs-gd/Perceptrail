package scan

import (
	"os"
	"path/filepath"
	"strconv"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// cheap preview: what the asset can show right now, without a transcode. Any size
// counts — the expensive stage brings the quality later. In order:
//  1. the main file itself, if the browser shows it (JPEG, PNG, …; H.264 video);
//  2. the biggest browser-viewable derivative of the group (the JPEG of a RAW);
//  3. a preview embedded in the main file (JpgFromRaw, PreviewImage, ThumbnailImage),
//     extracted by exiftool into the cache.
//
// Nothing found: the item waits for the expensive stage (Waiting).
type cheapPreview struct {
	logger  *l.Logger
	pool    *exiftoolPool
	dir     string // extracted previews: <dir>/<guid>/embedded.jpg
	extract func(tag, src, guid string) (string, error)
}

func NewCheapPreview(pool *exiftoolPool, cacheDir string, chin <-chan *flow.RawItem, chout chan<- *flow.RawItem, logger *l.Logger) chain.Processor {
	c := &cheapPreview{logger: logger, pool: pool, dir: filepath.Join(cacheDir, "previews")}
	c.extract = c.exiftoolExtract
	return chain.NewDecorator(chin, chout, c)
}

// Browser-viewable images; HEIC/HEIF (Safari only) and RAW are not
var viewableImage = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true,
	"image/webp": true, "image/avif": true, "image/bmp": true,
}

// H.264 plays everywhere; HEVC only in Safari — it waits for the transcode
var viewableVideoCodec = map[string]bool{"avc1": true, "avc3": true}

// Embedded previews, the biggest kind first
var embeddedPreviews = []string{"JpgFromRaw", "PreviewImage", "ThumbnailImage"}

func (c *cheapPreview) Decorate(it *flow.RawItem) (*flow.RawItem, error) {
	it.Item.PreviewPath, it.Item.PreviewMime = c.pick(it)
	return it, nil
}

func (c *cheapPreview) pick(it *flow.RawItem) (path, mime string) {
	main := it.Files[0]
	if viewable(main, it.Kinds[0], it.Exif[0]) {
		return main.Path, main.MimeType
	}

	best := -1
	for i := 1; i < len(it.Files); i++ {
		if it.Kinds[i] != flow.KindImage || !viewableImage[it.Files[i].MimeType] {
			continue
		}
		if best < 0 || pixels(it.Exif[i], it.Files[i]) > pixels(it.Exif[best], it.Files[best]) {
			best = i
		}
	}
	if best >= 0 {
		return it.Files[best].Path, it.Files[best].MimeType
	}

	for _, tag := range embeddedPreviews {
		if it.Exif[0] == nil || len(it.Exif[0][tag]) == 0 {
			continue
		}
		if p, err := c.extract(tag, main.Path, it.Item.Guid); err == nil {
			return p, "image/jpeg"
		} else {
			c.logger.Warn("Embedded preview not extracted", l.String("file", main.Path), l.String("tag", tag), l.Error(err))
		}
	}
	return "", ""
}

func viewable(f *dto.FileDto, kind flow.MediaKind, exif map[string][]byte) bool {
	switch kind {
	case flow.KindImage:
		return viewableImage[f.MimeType]
	case flow.KindVideo:
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

// exiftoolExtract writes the embedded preview tag of src to <dir>/<guid>/embedded.jpg
func (c *cheapPreview) exiftoolExtract(tag, src, guid string) (string, error) {
	dst := filepath.Join(c.dir, guid, "embedded.jpg")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if _, err := c.pool.Command("-b", "-"+tag, "-W!", dst, src); err != nil {
		return "", err
	}
	if info, err := os.Stat(dst); err != nil || info.Size() == 0 {
		return "", os.ErrNotExist
	}
	return dst, nil
}

func (c *cheapPreview) Stop() {
	if c.pool != nil {
		c.pool.Close()
	}
}
