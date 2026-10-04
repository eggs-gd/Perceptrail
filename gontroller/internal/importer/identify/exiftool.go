package identify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/eggs-gd/go-exiftool"
	"github.com/eggs-gd/perceplib/api"

	l "github.com/eggs-gd/perceplib/logger"
)

// Exiftool: what identify asks of exiftool. The stage starts its own (a pool of
// processes); the steps' own tests give a fake
type Exiftool interface {
	// Read: the tags of every path at once, as exiftool -n gives them (numbers as
	// numbers); one map per path, nil when the file could not be read
	Read(paths, tags []string) ([]api.RawExif, error)
	// Extract: src's embedded preview (tag) written to dst, src's Orientation copied
	// onto it
	Extract(tag, src, dst string) error
	// Close: the processes end (the stage's last step stops)
	Close()
}

// One file must not block an exiftool worker forever (broken or huge files)
const exiftoolTimeout = 2 * time.Minute

// pool: exiftool processes (-stay_open) for a pass, as many as identify's parallel
// readers; a command takes a free process. They start with the first command: a
// pass with nothing to read starts none.
type pool struct {
	count     int
	exec      string // the executable
	workers   []*exiftool.Server
	free      chan *exiftool.Server
	started   error // why the processes did not start: every command of the pass gets it
	logger    *l.Logger
	startOnce sync.Once
	closeOnce sync.Once
}

func newPool(count int, exec string, logger *l.Logger) *pool {
	return &pool{count: count, exec: exec, free: make(chan *exiftool.Server, count), logger: logger}
}

// start: every process, or none — one that cannot start (no exiftool, a wrong path)
// fails the pass's commands with its reason instead of taking the server down; the
// ones already started end
func (p *pool) start() {
	if p.exec != "" {
		exiftool.Exec = p.exec // the library takes its executable from a package variable
	}
	for range p.count {
		et, err := exiftool.NewServer()
		if err != nil {
			p.started = fmt.Errorf("exiftool %q can't start: %w", exiftool.Exec, err)
			for _, started := range p.workers {
				started.Close()
			}
			p.workers = nil
			return
		}
		et.SetTimeout(exiftoolTimeout)
		p.workers = append(p.workers, et)
	}
	for _, et := range p.workers {
		p.free <- et
	}
}

func (p *pool) command(args ...string) ([]byte, error) {
	p.startOnce.Do(p.start)
	if p.started != nil {
		return nil, p.started
	}
	et := <-p.free
	defer func() { p.free <- et }()
	return et.Command(args...)
}

// Read: one call for every path; -j gives an object per file read, with its path
// (SourceFile): a file it could not read has none
func (p *pool) Read(paths, tags []string) ([]api.RawExif, error) {
	args := []string{"-j", "-n"}
	for _, t := range tags {
		args = append(args, "-"+t)
	}
	out, err := p.command(slices.Concat(args, paths)...)
	if len(out) == 0 {
		return nil, err
	}
	byPath, uerr := decodeJSON(out)
	if uerr != nil {
		return nil, errors.Join(err, uerr)
	}
	res := make([]api.RawExif, len(paths))
	for i, path := range paths {
		res[i] = byPath[path]
	}
	return res, err
}

// Extract writes the embedded preview tag of src to dst. The embedded JPEG is stored
// as the sensor saw it, with no EXIF of its own; the RAW's Orientation is copied onto
// it, or a portrait shot shows on its side (the browser turns an <img> by its EXIF)
func (p *pool) Extract(tag, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if _, err := p.command("-b", "-"+tag, "-W!", dst, src); err != nil {
		return err
	}
	if info, err := os.Stat(dst); err != nil || info.Size() == 0 {
		return fmt.Errorf("%s: no %s written: %w", src, tag, os.ErrNotExist)
	}
	// No Orientation in the RAW is fine: the preview stays as it is
	if _, err := p.command("-overwrite_original", "-n", "-TagsFromFile", src, "-Orientation", dst); err != nil {
		p.logger.Debug("Orientation not copied to the embedded preview", l.String("file", src), l.Error(err))
	}
	return nil
}

// Close: the processes end — the stage's steps that use them stop (once)
func (p *pool) Close() {
	p.closeOnce.Do(func() {
		p.startOnce.Do(func() {}) // none starts after this
		for _, et := range p.workers {
			et.Close()
		}
	})
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
