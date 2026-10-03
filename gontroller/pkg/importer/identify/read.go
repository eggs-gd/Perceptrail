package identify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"perceptrail/gontroller/pkg/importer/discover"

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
// declared tags, numbers as numbers (-n: no print conversion; dates are unchanged). Extract reads the files (exiftool; a fake in tests): one map per
// path, nil when the file could not be read.
type Reader struct {
	logger  *l.Logger
	pool    *exiftoolPool
	args    []string // -j -<tag>…
	Extract func(paths []string) ([]api.RawExif, error)
}

// NewReader: tags are what the perceptors read; identify's own are added
func NewReader(pool *exiftoolPool, tags []string, logger *l.Logger) *Reader {
	r := &Reader{logger: logger, pool: pool, args: []string{"-j", "-n"}}
	seen := map[string]bool{}
	for _, t := range slices.Concat(ownTags, tags) {
		if !seen[t] {
			seen[t] = true
			r.args = append(r.args, "-"+t)
		}
	}
	r.Extract = r.exiftool
	return r
}

// Decorate starts the item of the group: its files and their metadata, every file
// of it at once (a keyed group: only the main file — the source knows the rest).
// A nil Exif: exiftool could not read the file. The item itself comes from validate.
func (e *Reader) Decorate(g discover.Group) (*draft, error) {
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
	res, err := e.Extract(paths)
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

// exiftool: one call for every path; -j gives an object per file read, with its
// path (SourceFile): a file it could not read has none
func (e *Reader) exiftool(paths []string) ([]api.RawExif, error) {
	out, err := e.pool.Command(slices.Concat(e.args, paths)...)
	if len(out) == 0 {
		return nil, err
	}
	byPath, uerr := decodeJSON(out)
	if uerr != nil {
		return nil, errors.Join(err, uerr)
	}
	res := make([]api.RawExif, len(paths))
	for i, p := range paths {
		res[i] = byPath[p]
	}
	return res, err
}

// decodeJSON: exiftool's -j output by SourceFile, every value as text: a number
// keeps its literal, a list is joined by ", "
func decodeJSON(out []byte) (map[string]api.RawExif, error) {
	var files []map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(out))
	if err := dec.Decode(&files); err != nil {
		return nil, fmt.Errorf("exiftool -j: %w", err)
	}
	byPath := make(map[string]api.RawExif, len(files))
	for _, f := range files {
		var src string
		if err := json.Unmarshal(f["SourceFile"], &src); err != nil {
			continue
		}
		delete(f, "SourceFile")
		m := make(api.RawExif, len(f))
		for k, raw := range f {
			m[k] = []byte(jsonText(raw))
		}
		byPath[src] = m
	}
	return byPath, nil
}

// jsonText: a JSON value as exiftool's text output prints it
func jsonText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		parts := make([]string, len(list))
		for i, v := range list {
			parts[i] = jsonText(v)
		}
		return strings.Join(parts, ", ")
	}
	return string(bytes.TrimSpace(raw)) // a number, true / false: its literal text
}

func (e *Reader) Stop() {
	if e.pool != nil {
		e.pool.Close()
	}
}
