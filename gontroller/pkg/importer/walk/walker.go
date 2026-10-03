// Package walk: the first step of the import — the library's files, one walk when
// asked (Next): every file with its stat, then the chain's flush. What a walk tells
// about the files it did not see (Gone) is its business too; when to walk again is
// the importer's.
package walk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

// Walker: the chain's entry point (a chain.Source). On every Next it walks the root
// once — every file with its stat — records the result (Last) and flushes the chain.
type Walker struct {
	logger *l.Logger
	root   string
	next   chan struct{}

	mu   sync.Mutex
	last Result
}

func New(root string, logger *l.Logger) *Walker {
	return &Walker{logger: logger, root: root, next: make(chan struct{}, 1)}
}

// Next asks for a walk (one is pending at most)
func (w *Walker) Next() {
	select {
	case w.next <- struct{}{}:
	default:
	}
}

// Last: the last finished walk
func (w *Walker) Last() Result {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.last
}

func (w *Walker) Run(ctx context.Context, out chain.Emitter[dto.ItemEntry]) {
	for {
		select {
		case <-w.next:
		case <-ctx.Done():
			return
		}
		r := w.walk(ctx, out.Emit)
		w.mu.Lock()
		w.last = r
		w.mu.Unlock()
		if !out.Flush() {
			return
		}
	}
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
