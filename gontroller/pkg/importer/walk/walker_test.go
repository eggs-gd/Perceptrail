package walk

import (
	"context"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/model/dto"
	"testing"
	"time"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

func newTestWalker(root string) *Walker {
	return New(root, l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
}

// runWalk collects what walk sends and returns it with the result
func runWalk(m *Walker) (Result, []string) {
	var paths []string
	result := m.walk(context.Background(), func(e dto.ItemEntry) bool {
		paths = append(paths, e.Path)
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
	result := newTestWalker(root).walk(ctx, func(dto.ItemEntry) bool {
		cancel() // take one file, then cancel
		return true
	})
	if result.Complete {
		t.Errorf("cancelled walk reported complete: %+v", result)
	}
}

// emitter: what the walker sends, flushes counted
type emitter struct{ flushes chan struct{} }

func (emitter) Emit(dto.ItemEntry) bool { return true }
func (e emitter) Flush() bool           { e.flushes <- struct{}{}; return true }

// The walker walks only when asked (Next), once per request; Last is that walk
func TestWalkerWalksOnNext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	w := newTestWalker(root)
	out := emitter{flushes: make(chan struct{})}
	go w.Run(t.Context(), out)

	select {
	case <-out.flushes:
		t.Fatal("walked before Next")
	case <-time.After(30 * time.Millisecond):
	}
	w.Next()
	select {
	case <-out.flushes:
	case <-time.After(5 * time.Second):
		t.Fatal("no walk after Next")
	}
	if r := w.Last(); !r.Complete || r.Files != 1 || r.Root != root {
		t.Errorf("last %+v", r)
	}
	select {
	case <-out.flushes:
		t.Fatal("walked again without Next")
	case <-time.After(30 * time.Millisecond):
	}
}

// Gone: only a complete walk that found files speaks; only under its root; never
// under an unreadable directory
func TestGone(t *testing.T) {
	file := func(p string) *dto.FileDto { return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: p}} }
	stale := []*dto.FileDto{file("/lib/a.jpg"), file("/lib/locked/b.jpg"), file("/other/c.jpg")}
	r := Result{Root: "/lib", Complete: true, Files: 3, Unreadable: []string{"/lib/locked"}}
	if g := Gone(r, stale); len(g) != 1 || g[0].Path != "/lib/a.jpg" {
		t.Errorf("gone %v", g)
	}
	if g := Gone(Result{Root: "/lib", Files: 3}, stale); g != nil {
		t.Error("an incomplete walk deleted files")
	}
	if g := Gone(Result{Root: "/lib", Complete: true}, stale); g != nil {
		t.Error("a walk that found nothing deleted files")
	}
}
