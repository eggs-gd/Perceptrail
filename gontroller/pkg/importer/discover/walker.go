package discover

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Walker: the chain's entry point (a chain.Source). It only reports what it finds —
// every file with its stat — then records the walk and flushes the chain. Grouping,
// MIME, the DB are later steps.
type Walker struct {
	logger   *l.Logger
	path     string
	interval time.Duration // pause after the work of a walk is done
	progress *Progress
}

// NewWalker walks the library again and again: interval after the chain has
// processed the previous walk (its flush reached the end), not after the walk itself
func NewWalker(path string, interval time.Duration, progress *Progress, logger *l.Logger) *Walker {
	return &Walker{logger: logger, path: path, interval: interval, progress: progress}
}

func (m *Walker) Run(ctx context.Context, out chain.Emitter[providers.Found]) {
	// Walk, flush, wait until the chain has processed that walk, pause, walk again
	for {
		m.progress.Walked(m.walk(ctx, out.Emit))
		if !out.Flush() || !m.progress.WaitIdle(ctx) {
			return
		}
		select {
		case <-time.After(m.interval):
		case <-ctx.Done():
			return
		}
	}
}

// walk sends every file under the root to chin. An unreadable subdirectory is
// skipped and recorded; an unreadable root or a cancel makes the walk incomplete.
func (m *Walker) walk(ctx context.Context, emit func(providers.Found) bool) Walk {
	result := Walk{Root: m.path, Started: time.Now()}

	if info, err := os.Stat(m.path); err != nil || !info.IsDir() {
		m.logger.Error("Library root is not a readable directory", l.String("path", m.path), l.Error(err))
		return result
	}

	err := filepath.WalkDir(m.path, func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if path == m.path {
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
		if !emit(providers.Found{Entry: dto.ItemEntry{
			Path: path, Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime(),
		}}) {
			return ctx.Err()
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, context.Canceled) {
			m.logger.Error("Walk failed", l.String("path", m.path), l.Error(err))
		}
		return result
	}
	result.Complete = true
	return result
}

// WalkOnce: one walk of root as the walker does it — every file with its stat —
// synchronously and once (tests of the whole import); the walk's result
func WalkOnce(ctx context.Context, root string, logger *l.Logger, each func(providers.Found)) Walk {
	m := &Walker{logger: logger, path: root}
	return m.walk(ctx, func(f providers.Found) bool { each(f); return true })
}
