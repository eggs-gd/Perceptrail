package discover

import (
	"context"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/importer/flow"
	"testing"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

func newTestMonitor(t *testing.T, root string) *fsMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &fsMonitor{
		logger: l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}),
		ctx:    ctx,
		cancel: cancel,
		path:   root,
	}
}

// runWalk collects what walk sends and returns it with the result
func runWalk(m *fsMonitor) (flow.WalkResult, []string) {
	ch := make(chan inType)
	res := make(chan flow.WalkResult)
	go func() {
		r := m.walk(ch)
		close(ch)
		res <- r
	}()

	var paths []string
	for in := range ch {
		paths = append(paths, in.path)
	}
	return <-res, paths
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

	result, paths := runWalk(newTestMonitor(t, root))
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

	result, paths := runWalk(newTestMonitor(t, root))
	if !result.Complete || result.Files != 2 {
		t.Errorf("got %+v, paths %v", result, paths)
	}
	if len(result.Unreadable) != 1 || result.Unreadable[0] != locked {
		t.Errorf("unreadable = %v, want [%s]", result.Unreadable, locked)
	}
}

func TestWalkMissingRoot(t *testing.T) {
	result, paths := runWalk(newTestMonitor(t, filepath.Join(t.TempDir(), "unmounted")))
	if result.Complete || len(paths) != 0 {
		t.Errorf("got %+v, paths %v", result, paths)
	}
}

func TestWalkCancelled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	writeFile(t, filepath.Join(root, "b.jpg"))

	m := newTestMonitor(t, root)
	ch := make(chan inType)
	res := make(chan flow.WalkResult)
	go func() { res <- m.walk(ch) }()
	<-ch // take one file, then cancel
	m.cancel()

	if result := <-res; result.Complete {
		t.Errorf("cancelled walk reported complete: %+v", result)
	}
}
