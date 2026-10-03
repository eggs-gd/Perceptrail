package identify

import (
	"os"
	"path/filepath"

	l "github.com/eggs-gd/perceplib/logger"
)

// Embedded: the embedded step's logic — when the group has nothing the browser shows,
// the preview embedded in the main file (JpgFromRaw, PreviewImage, ThumbnailImage —
// the biggest first) is extracted by exiftool into the cache. After validate: it needs
// the main file (classify) and the item's GUID (the cache directory). Extract writes
// one tag out (exiftool; a fake in tests).
type Embedded struct {
	logger  *l.Logger
	pool    *exiftoolPool
	dir     string // extracted previews: <dir>/<guid>/embedded.jpg
	Extract func(tag, src, guid string) (string, error)
}

func NewEmbedded(pool *exiftoolPool, cacheDir string, logger *l.Logger) *Embedded {
	e := &Embedded{logger: logger, pool: pool, dir: filepath.Join(cacheDir, "previews")}
	e.Extract = e.exiftoolExtract
	return e
}

// Embedded previews, the biggest kind first
var embeddedPreviews = []string{"JpgFromRaw", "PreviewImage", "ThumbnailImage"}

func (e *Embedded) Decorate(it *draft) (*draft, error) {
	if p, _ := viewableFile(it); p != "" {
		return it, nil // a file of the group shows: nothing to extract
	}
	for _, tag := range embeddedPreviews {
		if it.Exif[0] == nil || len(it.Exif[0][tag]) == 0 {
			continue
		}
		p, err := e.Extract(tag, it.Files[0].Path, it.Item.Guid)
		if err == nil {
			it.Embedded = p
			break
		}
		e.logger.Warn("Embedded preview not extracted", l.String("file", it.Files[0].Path), l.String("tag", tag), l.Error(err))
	}
	return it, nil
}

// exiftoolExtract writes the embedded preview tag of src to <dir>/<guid>/embedded.jpg.
// The embedded JPEG is stored as the sensor saw it, with no EXIF of its own; the
// RAW's Orientation is copied onto it, or a portrait shot shows on its side (the
// browser turns an <img> by its EXIF)
func (e *Embedded) exiftoolExtract(tag, src, guid string) (string, error) {
	dst := filepath.Join(e.dir, guid, "embedded.jpg")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if _, err := e.pool.Command("-b", "-"+tag, "-W!", dst, src); err != nil {
		return "", err
	}
	if info, err := os.Stat(dst); err != nil || info.Size() == 0 {
		return "", os.ErrNotExist
	}
	// No Orientation in the RAW is fine: the preview stays as it is
	if _, err := e.pool.Command("-overwrite_original", "-n", "-TagsFromFile", src, "-Orientation", dst); err != nil {
		e.logger.Debug("Orientation not copied to the embedded preview", l.String("file", src), l.Error(err))
	}
	return dst, nil
}

func (e *Embedded) Stop() {
	if e.pool != nil {
		e.pool.Close()
	}
}
