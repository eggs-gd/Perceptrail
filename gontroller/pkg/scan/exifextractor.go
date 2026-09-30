package scan

import (
	"fmt"
	"perceptrail/gontroller/pkg/scan/flow"
	"slices"

	"github.com/eggs-gd/go-exiftool"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

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

type exifExtractor struct {
	logger  *l.Logger
	extract func(path string) (api.RawExif, error)
	pool    *exiftoolPool
}

// NewExifExtractor: count steps on the same channels, sharing the exiftool pool
// (groups are independent, so they are read in parallel)
func NewExifExtractor(count int, pool *exiftoolPool, chin <-chan flow.FileGroup, chout chan<- *flow.RawItem, errch chan error, logger *l.Logger) chain.Processor {
	e := &exifExtractor{logger: logger, pool: pool}
	e.extract = e.exiftool

	workers := chain.NewChainProcessor(errch)
	for range count {
		workers.AddStep(chain.NewDecorator(chin, chout, e))
	}
	return workers
}

// Decorate starts the item of the group: its files and their metadata (a nil
// Exif: exiftool returned nothing). The item itself comes from the validator.
func (e *exifExtractor) Decorate(g flow.FileGroup) (*flow.RawItem, error) {
	files := g.Files
	out := &flow.RawItem{Files: files, Exif: make([]api.RawExif, len(files))}
	found := false
	for i, f := range files {
		res, err := e.extract(f.Path)
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

func (e *exifExtractor) exiftool(path string) (api.RawExif, error) {
	out, err := e.pool.Command(slices.Concat(allTags, []string{path})...)
	res := map[string][]byte{}
	if len(out) > 0 {
		if uerr := exiftool.Unmarshal(out, res); uerr != nil && err == nil {
			err = uerr
		}
	}
	return api.RawExif(res), err
}

func (e *exifExtractor) Stop() {
	if e.pool != nil {
		e.pool.Close()
	}
}
