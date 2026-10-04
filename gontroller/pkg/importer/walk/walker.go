// Package walk: the chain's entry — the library's files, one walk per pass. Every
// file it sees is a row of the files table, written now (path, stat, the time it was
// seen; Changed: new or its stat changed) and sent on; after a complete walk, the
// rows it did not see that it says are gone are sent too (Gone: the gate deletes
// them), then the chain's flush.
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

	"perceptrail/gontroller/pkg/model"
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

// Store: the files table as a walk writes and reads it
type Store interface {
	GetFileByPath(path string) (*dto.FileDto, error)
	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error)
}

// Walker: the walk step's logic (a chain.Source)
type Walker struct {
	logger *l.Logger
	root   string
	db     Store
}

// New: the chain's entry — out gets every file of a walk, then the gone ones
func New(db Store, root string, logger *l.Logger, out chan<- *dto.FileDto) chain.Processor {
	return chain.Entry(out, &Walker{logger: logger, root: root, db: db})
}

// Start: one walk — the files seen, then the gone ones
func (m *Walker) Start(ctx context.Context, emit func(*dto.FileDto) bool) error {
	r := m.walk(ctx, func(e dto.ItemEntry) bool {
		f, err := m.seen(e, time.Now())
		if err != nil {
			m.logger.Error("File not stored", l.String("path", e.Path), l.Error(err))
			return true
		}
		return emit(f)
	})
	gone := m.gone(r)
	for _, f := range gone {
		f.Gone = true
		if !emit(f) {
			break
		}
	}
	if r.Complete {
		m.logger.Info("Walk complete", l.Int("files", r.Files), l.Int("unreadable", len(r.Unreadable)), l.Int("gone", len(gone)))
	}
	return nil
}

// seen: the file's row — created, or its stat refreshed; stamped as seen at now.
// Changed: new, or its size / mtime differ.
func (m *Walker) seen(e dto.ItemEntry, now time.Time) (*dto.FileDto, error) {
	f, err := m.db.GetFileByPath(e.Path)
	switch {
	case errors.Is(err, model.ErrNotFound):
		if f, err = m.db.CreateFile(e); err != nil {
			return nil, err
		}
		f.Changed = true
	case err != nil:
		return nil, err
	case !f.ModTime.Equal(e.ModTime) || f.Size != e.Size:
		// The fresh stat stored, or every walk sees the file as changed again (and
		// the short hash would use the stale size)
		f.Size, f.ModTime = e.Size, e.ModTime
		f.Changed = true
	}
	f.CheckTime = now
	_, err = m.db.UpdateFiles([]*dto.FileDto{f})
	return f, err
}

// gone: the rows the walk did not stamp that it says are deleted (Gone)
func (m *Walker) gone(r Result) []*dto.FileDto {
	switch {
	case !r.Complete:
		m.logger.Warn("Walk incomplete: deletions are not checked")
		return nil
	case r.Files == 0:
		m.logger.Warn("Walk found no files: deletions are not checked", l.String("path", r.Root))
		return nil
	}
	stale, err := m.db.GetFilesCheckedBefore(r.Started)
	if err != nil {
		m.logger.Error("Deletions: can't read files", l.Error(err))
		return nil
	}
	return Gone(r, stale)
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

// Gone: of the files a walk did not stamp (stale), the ones it says are deleted —
// only after a complete walk that found files, only under its root (another root:
// the config changed, not ours to judge), never under an unreadable directory, never
// a file it saw (stamped as it went)
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
