package identify

import (
	"fmt"
	"slices"

	"perceptrail/gontroller/pkg/importer/gate"

	"github.com/eggs-gd/perceplib/api"

	l "github.com/eggs-gd/perceplib/logger"
)

// ownTags: what identify itself reads of every file — the kind (classify), a broken
// file (validate), the sizes and the codec (sizes, pick), the embedded previews
// (only whether they are there: their bytes are extracted by embedded)
var ownTags = []string{
	"MIMEType", "Error",
	"ImageWidth", "ImageHeight", "ImageSize", "ExifImageWidth", "CompressorID",
	"JpgFromRaw", "PreviewImage", "ThumbnailImage",
}

// Reader: the read step's logic — one exiftool call for the whole group, only the
// declared tags
type Reader struct {
	logger *l.Logger
	tool   Exiftool
	tags   []string // identify's own and the perceptors'
}

// NewReader: tags are what the perceptors read; identify's own are added
func NewReader(tool Exiftool, tags []string, logger *l.Logger) *Reader {
	r := &Reader{logger: logger, tool: tool}
	seen := map[string]bool{}
	for _, t := range slices.Concat(ownTags, tags) {
		if !seen[t] {
			seen[t] = true
			r.tags = append(r.tags, t)
		}
	}
	return r
}

// Decorate starts the item of the group: its files and their metadata, every file
// of it at once (a keyed group: only the main file — the source knows the rest).
// A nil Exif: exiftool could not read the file. The item itself comes from validate.
func (e *Reader) Decorate(g gate.Group) (*draft, error) {
	files := g.Files
	out := &draft{Files: files, Exif: make([]api.RawExif, len(files)), Key: g.Key, Show: g.Show, Meta: g.Meta, MetaHash: g.MetaHash, Kind: g.Kind}
	read := files
	if g.Key != "" {
		read = files[:1]
	}
	paths := make([]string, len(read))
	for i, f := range read {
		paths[i] = f.Path
	}
	res, err := e.tool.Read(paths, e.tags)
	found := false
	for i := range read {
		if i < len(res) && res[i] != nil {
			out.Exif[i] = res[i]
			found = true
			continue
		}
		// Not read: classify falls back to the extension for this file; validate skips
		// the group if this turns out to be the main file
		e.logger.Warn("exiftool: no metadata", l.String("file", paths[i]), l.Error(err))
	}
	if !found {
		return nil, fmt.Errorf("exiftool: no metadata for the group of %s: %v", files[0].Path, err)
	}
	if err != nil {
		// ExifTool reported a problem but still returned data: use it
		e.logger.Warn("exiftool reported a problem", l.String("file", files[0].Path), l.Error(err))
	}
	return out, nil
}

func (e *Reader) Stop() { closeTool(e.tool) }

// closeTool: the stage's own exiftool ends when a step using it stops (a test's fake
// has nothing to close)
func closeTool(tool Exiftool) {
	if c, ok := tool.(interface{ Close() }); ok {
		c.Close()
	}
}
