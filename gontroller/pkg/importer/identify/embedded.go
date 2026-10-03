package identify

import (
	"path/filepath"

	l "github.com/eggs-gd/perceplib/logger"
)

// Embedded: the embedded step's logic — when the group has nothing the browser shows,
// the preview embedded in the main file (JpgFromRaw, PreviewImage, ThumbnailImage —
// the biggest first) is extracted by exiftool into the cache. After validate: it needs
// the main file (classify) and the item's GUID (the cache directory).
type Embedded struct {
	logger *l.Logger
	tool   Exiftool
	dir    string // extracted previews: <dir>/<guid>/embedded.jpg
}

func NewEmbedded(tool Exiftool, cacheDir string, logger *l.Logger) *Embedded {
	return &Embedded{logger: logger, tool: tool, dir: filepath.Join(cacheDir, "previews")}
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
		dst := filepath.Join(e.dir, it.Item.Guid, "embedded.jpg")
		err := e.tool.Extract(tag, it.Files[0].Path, dst)
		if err == nil {
			it.Embedded = dst
			break
		}
		e.logger.Warn("Embedded preview not extracted", l.String("file", it.Files[0].Path), l.String("tag", tag), l.Error(err))
	}
	return it, nil
}

func (e *Embedded) Stop() { closeTool(e.tool) }
