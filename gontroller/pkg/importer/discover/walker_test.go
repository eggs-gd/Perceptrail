package discover

import (
	"context"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/providers"
	"testing"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

func newTestWalker(root string) *Walker {
	return NewWalker(root, 0, NewProgress(), l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
}

// runWalk collects what walk sends and returns it with the result
func runWalk(m *Walker) (Walk, []string) {
	var paths []string
	result := m.walk(context.Background(), func(f providers.Found) bool {
		paths = append(paths, f.Entry.Path)
		return true
	})
	return result, paths
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkComplete(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	writeFile(t, filepath.Join(root, "sub", "b.jpg"))

	result, paths := runWalk(newTestWalker(root))
	if !result.Complete || result.Files != 2 || len(paths) != 2 || len(result.Unreadable) != 0 {
		t.Errorf("got %+v, paths %v", result, paths)
	}
}

// One unreadable directory must not stop the walk (and must be reported, so its
// files are not taken as deleted)
func TestWalkSkipsUnreadableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	locked := filepath.Join(root, "locked")
	writeFile(t, filepath.Join(locked, "hidden.jpg"))
	writeFile(t, filepath.Join(root, "z", "c.jpg"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	result, paths := runWalk(newTestWalker(root))
	if !result.Complete || result.Files != 2 {
		t.Errorf("got %+v, paths %v", result, paths)
	}
	if len(result.Unreadable) != 1 || result.Unreadable[0] != locked {
		t.Errorf("unreadable = %v, want [%s]", result.Unreadable, locked)
	}
}

func TestWalkMissingRoot(t *testing.T) {
	result, paths := runWalk(newTestWalker(filepath.Join(t.TempDir(), "unmounted")))
	if result.Complete || len(paths) != 0 {
		t.Errorf("got %+v, paths %v", result, paths)
	}
}

func TestWalkCancelled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	writeFile(t, filepath.Join(root, "b.jpg"))

	ctx, cancel := context.WithCancel(t.Context())
	result := newTestWalker(root).walk(ctx, func(providers.Found) bool {
		cancel() // take one file, then cancel
		return true
	})
	if result.Complete {
		t.Errorf("cancelled walk reported complete: %+v", result)
	}
}
