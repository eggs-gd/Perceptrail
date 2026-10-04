package walk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"perceptrail/gontroller/internal/model/dto"
	"slices"
	"testing"
	"time"

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

// rows: the files table in memory; writes counts the rows written
type rows struct {
	byPath map[string]*dto.FileDto
	writes int
}

func (r *rows) GetAllFiles() ([]*dto.FileDto, error) {
	var out []*dto.FileDto
	for _, f := range r.byPath {
		c := *f
		out = append(out, &c)
	}
	return out, nil
}
func (r *rows) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	var out []*dto.FileDto
	for _, e := range entries {
		f := &dto.FileDto{ItemEntry: e, Changed: true}
		c := *f
		r.byPath[e.Path] = &c
		out = append(out, f)
	}
	r.writes += len(entries)
	return out, nil
}
func (r *rows) SaveStats(fs []*dto.FileDto) error {
	for _, f := range fs {
		row := r.byPath[f.Path]
		row.Size, row.ModTime, row.Changed = f.Size, f.ModTime, f.Changed
	}
	r.writes += len(fs)
	return nil
}

// files: the end of a test chain — what it got, by name
type files struct{ got []string }

func (f *files) Consume(r dto.WalkedFile) error {
	mark := ""
	switch {
	case r.Missing:
		mark = " gone"
	case r.Changed:
		mark = " changed"
	}
	f.got = append(f.got, r.Name+mark)
	return nil
}

// A pass: every file as a row (new or changed: Changed), then the rows it did not
// see (Missing); then its output closes (the pass ends)
func TestWalkPass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jpg"))
	writeFile(t, filepath.Join(root, "b.jpg"))
	db := &rows{byPath: map[string]*dto.FileDto{}}
	pass := func() []string {
		found := make(chan dto.WalkedFile)
		got := &files{}
		c := chain.NewChainProcessor(nil)
		c.AddStep(New(db, root, nil, testLogger, found))
		c.AddStep(chain.NewEnd(found, got))
		c.Process(t.Context())
		return got.got
	}
	if g := pass(); !slices.Equal(g, []string{"a.jpg changed", "b.jpg changed"}) {
		t.Errorf("first pass %v", g)
	}
	os.Remove(filepath.Join(root, "b.jpg"))
	// a.jpg's change stays pending (nothing decided its group): still changed
	db.writes = 0
	if g := pass(); !slices.Equal(g, []string{"a.jpg changed", "b.jpg gone"}) {
		t.Errorf("second pass %v", g)
	}
	if db.writes != 0 {
		t.Errorf("an unchanged file was written: %d writes", db.writes)
	}
	// A new stat is saved (and marks the change); the rest of the row stays
	db.byPath[filepath.Join(root, "a.jpg")].Changed = false // its group decided
	db.byPath[filepath.Join(root, "a.jpg")].Role = "original"
	writeFile(t, filepath.Join(root, "a.jpg"))
	later := time.Now().Add(time.Hour)
	os.Chtimes(filepath.Join(root, "a.jpg"), later, later)
	if g := pass(); !slices.Equal(g, []string{"a.jpg changed", "b.jpg gone"}) {
		t.Errorf("third pass %v", g)
	}
	if a := db.byPath[filepath.Join(root, "a.jpg")]; !a.ModTime.Equal(later) || !a.Changed || a.Role != "original" {
		t.Errorf("a.jpg's row %+v", a)
	}
}

// More files than a page: every file once, in the walk's (name) order — the plain
// folder's grouper relies on it
func TestWalkPages(t *testing.T) {
	root := t.TempDir()
	var want []string
	for i := range page*2 + 3 {
		name := fmt.Sprintf("f%04d.jpg", i)
		writeFile(t, filepath.Join(root, name))
		want = append(want, name+" changed")
	}
	db := &rows{byPath: map[string]*dto.FileDto{}}
	found := make(chan dto.WalkedFile)
	got := &files{}
	c := chain.NewChainProcessor(nil)
	c.AddStep(New(db, root, nil, testLogger, found))
	c.AddStep(chain.NewEnd(found, got))
	c.Process(t.Context())
	if !slices.Equal(got.got, want) {
		t.Errorf("got %d files, want %d in order", len(got.got), len(want))
	}
}

// Missing: only a complete walk that found files speaks; only under its root; never
// under an unreadable directory
func TestMissing(t *testing.T) {
	file := func(p string) *dto.FileDto { return &dto.FileDto{Path: p} }
	stale := []*dto.FileDto{file("/lib/a.jpg"), file("/lib/locked/b.jpg"), file("/other/c.jpg")}
	r := Result{Root: "/lib", Complete: true, Files: 3, Unreadable: []string{"/lib/locked"}}
	if g := Missing(r, stale); len(g) != 1 || g[0].Path != "/lib/a.jpg" {
		t.Errorf("gone %v", g)
	}
	if g := Missing(Result{Root: "/lib", Files: 3}, stale); g != nil {
		t.Error("an incomplete walk deleted files")
	}
	if g := Missing(Result{Root: "/lib", Complete: true}, stale); g != nil {
		t.Error("a walk that found nothing deleted files")
	}
}
