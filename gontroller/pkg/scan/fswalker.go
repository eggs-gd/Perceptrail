package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// fswalker: the entry point. It only reports what it finds — every file with its
// stat, then the end-of-walk marker. Grouping, MIME, the DB are later steps.

type inType struct {
	path string
	info os.DirEntry
	// Set only on the end-of-walk marker
	done *walkResult
}

type fsMonitor struct {
	logger *l.Logger
	ctx    context.Context
	cancel context.CancelFunc

	mu sync.Mutex // guards cancel: Stop is called from two goroutines

	path string
}

func NewFsWalker(path string, chout chan<- fileEvent, logger *l.Logger) chain.Processor {
	return chain.NewEntryPoint(chout, &fsMonitor{logger: logger, path: path})
}

func (m *fsMonitor) Start(chin chan<- inType, ctx context.Context) {
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()
	defer m.Stop()

	result := m.walk(chin)

	select {
	case chin <- inType{done: &result}:
	case <-m.ctx.Done():
	}
}

// walk sends every file under the root to chin. An unreadable subdirectory is
// skipped and recorded; an unreadable root or a cancel makes the walk incomplete.
func (m *fsMonitor) walk(chin chan<- inType) walkResult {
	result := walkResult{root: m.path, started: time.Now()}

	if info, err := os.Stat(m.path); err != nil || !info.IsDir() {
		m.logger.Error("Library root is not a readable directory", l.String("path", m.path), l.Error(err))
		return result
	}

	err := filepath.WalkDir(m.path, func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := m.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if path == m.path {
				return err
			}
			m.logger.Warn("Unreadable, skipped", l.String("path", path), l.Error(err))
			result.unreadable = append(result.unreadable, path)
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		select {
		case chin <- inType{path: path, info: entry}:
			result.files++
			return nil
		case <-m.ctx.Done():
			return m.ctx.Err()
		}
	})

	if err != nil {
		if !errors.Is(err, context.Canceled) {
			m.logger.Error("Walk failed", l.String("path", m.path), l.Error(err))
		}
		return result
	}
	result.complete = true
	return result
}

func (m *fsMonitor) Stop() {
	// Called from both the walk goroutine and the chain runner
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// Decorate adds the stat. A file that vanished since it was listed is skipped: it
// is not seen, so the gate will take it as deleted.
func (m *fsMonitor) Decorate(in inType) (fileEvent, error) {
	if in.done != nil {
		return fileEvent{done: in.done}, nil
	}
	info, err := in.info.Info()
	if err != nil {
		return fileEvent{}, chain.ErrSkippedItem
	}
	return fileEvent{entry: dto.ItemEntry{
		Path:    in.path,
		Name:    in.info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}}, nil
}
