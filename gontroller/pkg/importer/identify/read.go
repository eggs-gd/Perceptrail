package identify

import (
	"fmt"
	"perceptrail/gontroller/pkg/importer/flow"
	"slices"

	"github.com/eggs-gd/go-exiftool"

	"github.com/eggs-gd/perceplib/api"

	l "github.com/eggs-gd/perceplib/logger"
)

// exif: exiftool for every file of the group. The main file is not known yet (mime
// ranks the group from this metadata), so every file gets the full set; the main
// file's set is what the short hash is computed from, as before.

// exiftool -all --ExifToolVersion -s2 ./_D3A9906.JPG
var allTags []string = []string{
	"-all",
	"--ExifToolVersion",
	"-s2",
}

// Reader: the read step's logic. Extract reads one file (exiftool; a fake in tests).
type Reader struct {
	logger  *l.Logger
	pool    *exiftoolPool
	Extract func(path string) (api.RawExif, error)
}

func NewReader(pool *exiftoolPool, logger *l.Logger) *Reader {
	r := &Reader{logger: logger, pool: pool}
	r.Extract = r.exiftool
	return r
}

// Decorate starts the item of the group: its files and their metadata (a nil
// Exif: exiftool returned nothing). The item itself comes from the validator.
func (e *Reader) Decorate(g flow.FileGroup) (*flow.RawItem, error) {
	files := g.Files
	out := &flow.RawItem{Files: files, Exif: make([]api.RawExif, len(files)), Key: g.Key, Show: g.Show, Meta: g.Meta, MetaHash: g.MetaHash, Kind: g.Kind}
	found := false
	for i, f := range files {
		if g.Key != "" && i > 0 {
			break // a keyed group: the grouper knows the files, only the main one is read
		}
		res, err := e.Extract(f.Path)
		if len(res) == 0 {
			// Nothing usable: mime falls back to the extension for this file; the
			// validator skips the group if this turns out to be the main file
			e.logger.Warn("exiftool: no metadata", l.String("file", f.Path), l.Error(err))
			continue
		}
		if err != nil {
			// ExifTool reported a problem but still returned data: use it
			e.logger.Warn("exiftool reported a problem", l.String("file", f.Path), l.Error(err))
		}
		out.Exif[i] = res
		found = true
	}
	if !found {
		return nil, fmt.Errorf("exiftool: no metadata for the group of %s", files[0].Path)
	}
	return out, nil
}

func (e *Reader) exiftool(path string) (api.RawExif, error) {
	out, err := e.pool.Command(slices.Concat(allTags, []string{path})...)
	res := map[string][]byte{}
	if len(out) > 0 {
		if uerr := exiftool.Unmarshal(out, res); uerr != nil && err == nil {
			err = uerr
		}
	}
	return api.RawExif(res), err
}

func (e *Reader) Stop() {
	if e.pool != nil {
		e.pool.Close()
	}
}
