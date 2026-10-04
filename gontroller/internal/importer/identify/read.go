package identify

import (
	"fmt"
	"slices"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"

	l "github.com/eggs-gd/perceplib/logger"
)

// ownTags: what identify itself reads of every file — the kind (classify), a broken
// file (validate), the sizes and the codec, the embedded previews (only whether
// they are there: show extracts their bytes when it needs one)
var ownTags = []string{
	"MIMEType", "Error",
	"ImageWidth", "ImageHeight", "ImageSize", "ExifImageWidth", "CompressorID",
	"JpgFromRaw", "PreviewImage", "ThumbnailImage",
}

// reader: the read step's logic — everything known of the group without the DB, so
// groups go in parallel: its metadata (one exiftool call, only the declared tags),
// the kinds and the main file (classify), the metadata package (merge), the main
// file's fingerprint
type reader struct {
	logger   *l.Logger
	tool     Exiftool
	tags     []string        // identify's own and the perceptors'
	declared map[string]bool // the perceptors': what makes the package
}

// newReader: declared are the tags the perceptors read; identify's own are added
func newReader(tool Exiftool, declared []string, logger *l.Logger) *reader {
	r := &reader{logger: logger, tool: tool, declared: map[string]bool{}}
	for _, t := range declared {
		r.declared[t] = true
	}
	seen := map[string]bool{}
	for _, t := range slices.Concat(ownTags, declared) {
		if !seen[t] {
			seen[t] = true
			r.tags = append(r.tags, t)
		}
	}
	return r
}

func (r *reader) Decorate(g dto.Asset) (*draft, error) {
	d, err := r.read(g)
	if err != nil {
		return nil, err
	}
	classify(d)
	merge(d, r.declared)
	if d.isMedia() { // else validate ignores it: nothing to identify
		if d.Hash, err = fingerprint(d.Files[0].Path); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// read: the group's files and their metadata, every file of it at once (a keyed
// group: only the main file — the source knows the rest). A nil Exif: exiftool
// could not read the file.
func (e *reader) read(g dto.Asset) (*draft, error) {
	files := g.Files
	out := &draft{Asset: g, Exif: make([]api.RawExif, len(files))}
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
