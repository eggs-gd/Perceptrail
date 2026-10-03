// Package walk: the first step of the import — the library's files, a walk per
// request: every file with its stat, then the chain's flush; the walk's result goes
// to whoever judges the deletions on that flush (the gate). When a walk comes is the
// chain's owner's business.
package walk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Result describes a finished walk. Deletions may be derived from it only if the
// walk was complete: a cancelled walk or an unreadable root says nothing about which
// files are gone.
type Result struct {
	Root    string
	Started time.Time
	// The walk reached the end
	Complete bool
	// Files seen (before grouping and filtering)
	Files int
	// Directories that could not be read: their files are not "deleted"
	Unreadable []string
}

// Store: what a walk asks of the model — the files it did not stamp, what their
// being gone means
type Store interface {
	GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error)
	Gone(files []*dto.FileDto) (deleted, dirty int, err error)
}

// Walker: the walk step's logic (a chain.Spreader: a request in, the files out)
type Walker struct {
	logger *l.Logger
	root   string
	result *Result
}

// New: in — a walk request; out — every file found, then the flush. result gets the
// walk's result before the flush (the owner shares it with the gate)
func New(root string, result *Result, logger *l.Logger, in *chain.Pipe[struct{}], out *chain.Pipe[dto.ItemEntry]) chain.Processor {
	return chain.Spread(in, out, &Walker{logger: logger, root: root, result: result})
}

func (m *Walker) Spread(ctx context.Context, _ struct{}, emit func(dto.ItemEntry) bool) error {
	*m.result = m.walk(ctx, emit)
	return nil
}

func (m *Walker) walk(ctx context.Context, emit func(dto.ItemEntry) bool) Result {
	result := Result{Root: m.root, Started: time.Now()}

	if info, err := os.Stat(m.root); err != nil || !info.IsDir() {
		m.logger.Error("Library root is not a readable directory", l.String("path", m.root), l.Error(err))
		return result
	}

	err := filepath.WalkDir(m.root, func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if path == m.root {
				return err
			}
			m.logger.Warn("Unreadable, skipped", l.String("path", path), l.Error(err))
			result.Unreadable = append(result.Unreadable, path)
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		result.Files++
		// A file that vanished since it was listed is not seen: the gate will take it
		// as deleted
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if !emit(dto.ItemEntry{Path: path, Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime()}) {
			return ctx.Err()
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, context.Canceled) {
			m.logger.Error("Walk failed", l.String("path", m.root), l.Error(err))
		}
		return result
	}
	result.Complete = true
	return result
}

// Once: one walk of root as the walker does it — every file with its stat —
// synchronously (tests of the whole import); the walk's result
func Once(ctx context.Context, root string, logger *l.Logger, each func(dto.ItemEntry)) Result {
	m := &Walker{logger: logger, root: root}
	return m.walk(ctx, func(e dto.ItemEntry) bool { each(e); return true })
}

// Delete: the files the walk did not see that it says are gone (Gone) go by the
// model's rules (a main file's item deleted, a sidecar's item processed again)
func Delete(db Store, r Result, logger *l.Logger) {
	switch {
	case !r.Complete:
		logger.Warn("Walk incomplete: deletions are not checked")
		return
	case r.Files == 0:
		logger.Warn("Walk found no files: deletions are not checked", l.String("path", r.Root))
		return
	}
	logger.Info("Walk complete", l.Int("files", r.Files), l.Int("unreadable", len(r.Unreadable)))
	stale, err := db.GetFilesCheckedBefore(r.Started)
	if err != nil {
		logger.Error("Deletions: can't read files", l.Error(err))
		return
	}
	gone := Gone(r, stale)
	deleted, dirty, err := db.Gone(gone)
	if err != nil {
		logger.Error("Deletions failed", l.Error(err))
		return
	}
	logger.Info("Deletions", l.Int("files", len(gone)), l.Int("items", deleted), l.Int("dirty", dirty))
}

// Gone: of the files a walk did not stamp (stale), the ones it says are deleted —
// only after a complete walk that found files, only under its root (another root:
// the config changed, not ours to judge), never under an unreadable directory, never
// a file a grouper held back (seen, its asset not complete yet: the gate stamps
// those anyway)
func Gone(r Result, stale []*dto.FileDto) []*dto.FileDto {
	if !r.Complete || r.Files == 0 {
		return nil // an empty root (an unmounted drive's mount point) must not delete the library
	}
	var gone []*dto.FileDto
	for _, f := range stale {
		if !isUnder(f.Path, r.Root) {
			continue
		}
		if slices.ContainsFunc(r.Unreadable, func(dir string) bool { return f.Path == dir || isUnder(f.Path, dir) }) {
			continue
		}
		gone = append(gone, f)
	}
	return gone
}

func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
