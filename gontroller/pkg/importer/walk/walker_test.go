package walk

import (
	"context"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/model/dto"
	"slices"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

var testLogger = l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})

func newTestWalker(root string) *Walker {
	return &Walker{logger: testLogger, root: root}
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

// rows: the files table in memory (Changed, Gone are not stored)
type rows struct{ byPath map[string]*dto.FileDto }

func (r *rows) GetFileByPath(path string) (*dto.FileDto, error) {
	if f, ok := r.byPath[path]; ok {
		c := *f
		c.Changed, c.Gone = false, false
		return &c, nil
	}
	return nil, model.ErrNotFound
}
func (r *rows) CreateFile(e dto.ItemEntry) (*dto.FileDto, error) {
	f := &dto.FileDto{ItemEntry: e}
	r.byPath[e.Path] = f
	return f, nil
}
func (r *rows) UpdateFiles(fs []*dto.FileDto) ([]*dto.FileDto, error) {
	for _, f := range fs {
		c := *f
		r.byPath[f.Path] = &c
	}
	return fs, nil
}
func (r *rows) GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error) {
	var out []*dto.FileDto
	for _, f := range r.byPath {
		if f.CheckTime.Before(t) {
			c := *f
			c.Changed, c.Gone = false, false
			out = append(out, &c)
		}
	}
	return out, nil
}

// files: the end of a test chain — what it got, by name
type files struct{ got []string }

func (f *files) Consume(r *dto.FileDto) error {
	mark := ""
	switch {
	case r.Gone:
		mark = " gone"
	case r.Changed:
		mark = " changed"
	}
	f.got = append(f.got, r.Name+mark)
	return nil
}

// A pass: every file as a row (new or changed: Changed), then the rows it did not
// see (Gone), then the flush (the pass ends)
func TestWalkPass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	writeFile(t, filepath.Join(root, "b.jpg"))
	found := chain.NewPipe[*dto.FileDto](0)
	got := &files{}
	c := chain.New(nil)
	c.AddStep(New(&rows{byPath: map[string]*dto.FileDto{}}, root, testLogger, found))
	c.AddStep(chain.End[*dto.FileDto](found, got))
	go c.Process(t.Context())
	pass := func() []string {
		got.got = nil
		if !c.Run(t.Context()) {
			t.Fatal("the pass did not end")
		}
		return got.got
	}
	if g := pass(); !slices.Equal(g, []string{"a.jpg changed", "b.jpg changed"}) {
		t.Errorf("first pass %v", g)
	}
	os.Remove(filepath.Join(root, "b.jpg"))
	if g := pass(); !slices.Equal(g, []string{"a.jpg", "b.jpg gone"}) {
		t.Errorf("second pass %v", g)
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
