package importer_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/importer"
	"perceptrail/gontroller/internal/library"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/perceptor"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// One sqlite database for the package (model keeps a single connection); every test
// uses its own library root, and deletions are scoped to the root. The tests read
// and write it through the whole model; the import's steps see their own part.
var testDB *model.Proxy

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gontroller-import-test")
	if err != nil {
		panic(err)
	}
	// The server's own start: a config file (the database in dir), the model, the
	// perceptors, the libraries (Apple, the plain folder last)
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), nil, 0o644); err != nil {
		panic(err)
	}
	cfg, err := config.Read(filepath.Join(dir, "config.yml"))
	if err != nil {
		panic(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	if testDB, err = model.Open(cfg, logger); err != nil {
		panic(err)
	}
	if err := perceptor.Load(cfg, logger); err != nil {
		panic(err)
	}
	if err := library.Enable(cfg, testDB, logger); err != nil {
		panic(err)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// scan runs one pass of the import (the service's Pass) over root, with a real
// exiftool; the main files of the items published in it. A new chain each pass:
// the groupers start empty, as on a restart.
func scan(t *testing.T, root string) []string {
	t.Helper()
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	start := time.Now()
	if err := importer.New(pass{root: root, cache: t.TempDir()}, testDB, logger).Pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	var published []string
	err := testDB.StreamItemsSince(start.Add(-time.Second), func(it *dto.ItemDto, _ []*dto.FileDto) error {
		if !it.DeletedAt.Valid && !it.UpdatedAt.Before(start) {
			published = append(published, it.Path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(published)
	return published
}

// pass: what the import reads of the config, for one test's library
type pass struct{ root, cache string }

func (p pass) LibraryRoot() string   { return p.root }
func (p pass) CacheDir() string      { return p.cache }
func (p pass) Rescan() time.Duration { return time.Minute }
func (p pass) Exiftool() string      { return "exiftool" }

func itemAt(t *testing.T, path string) *dto.ItemDto {
	t.Helper()
	item, err := testDB.GetItemByPath(path)
	if err != nil {
		t.Fatalf("no item at %s: %v", path, err)
	}
	return item
}

func assertNoItem(t *testing.T, guid string) {
	t.Helper()
	if _, err := testDB.GetItemByGuid(guid); err == nil {
		t.Errorf("item %s still visible", guid)
	}
}

func TestValidatorLifecycle(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a.jpg"), filepath.Join(root, "b.jpg")
	write(t, a, "photo a")
	write(t, b, "photo b")

	// New
	if got := scan(t, root); len(got) != 2 {
		t.Fatalf("new: processed %v", got)
	}
	guidA, guidB := itemAt(t, a).Guid, itemAt(t, b).Guid

	// Same: nothing to do
	if got := scan(t, root); len(got) != 0 {
		t.Errorf("same: processed %v", got)
	}

	// Changed: same path, new content → same item, new hash, processed again
	hashA := itemAt(t, a).HashShort
	write(t, a, "photo a, edited")
	if got := scan(t, root); len(got) != 1 || got[0] != a {
		t.Errorf("changed: processed %v", got)
	}
	if item := itemAt(t, a); item.Guid != guidA || item.HashShort == hashA || item.State != dto.Visible {
		t.Errorf("changed: %+v", item)
	}

	// Moved: the item keeps its GUID and gets the new path
	moved := filepath.Join(root, "sub", "b-renamed.jpg")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(b, moved); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	if item := itemAt(t, moved); item.Guid != guidB {
		t.Errorf("moved: GUID %s, want %s", item.Guid, guidB)
	}
	if f, err := testDB.GetFileByPath(moved); err != nil || f.GUID != guidB || f.LinkedTo != guidB {
		t.Errorf("moved: file row %+v, %v", f, err)
	}
	if _, err := testDB.GetFileByPath(b); err == nil {
		t.Error("moved: the old file row is still there")
	}

	// Duplicate: same content, the original still exists → a second item
	dup := filepath.Join(root, "b-copy.jpg")
	write(t, dup, "photo b")
	scan(t, root)
	if item := itemAt(t, dup); item.Guid == guidB {
		t.Error("duplicate: took over the original's item")
	}
	if item := itemAt(t, moved); item.Guid != guidB {
		t.Error("duplicate: the original lost its item")
	}

	// Deleted main file: the item disappears
	dupGuid := itemAt(t, dup).Guid
	if err := os.Remove(dup); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	assertNoItem(t, dupGuid)
	if _, err := testDB.GetFileByPath(dup); err == nil {
		t.Error("deleted: the file row is still there")
	}
}

func TestValidatorSidecarDeleted(t *testing.T) {
	root := t.TempDir()
	photo, xmp := filepath.Join(root, "p.jpg"), filepath.Join(root, "p.xmp")
	write(t, photo, "photo")
	write(t, xmp, "sidecar")
	scan(t, root)
	guid := itemAt(t, photo).Guid

	if err := os.Remove(xmp); err != nil {
		t.Fatal(err)
	}
	// The gone sidecar makes the item Dirty before the grouper's flush gives the
	// photo's group: processed in the same walk, though the photo did not change
	if got := scan(t, root); len(got) != 1 || got[0] != photo {
		t.Errorf("sidecar gone: processed %v", got)
	}
	if item := itemAt(t, photo); item.Guid != guid || item.State != dto.Visible {
		t.Errorf("sidecar gone: %+v, want the same item, Visible", item)
	}
}

func TestValidatorKeepsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	hidden := filepath.Join(locked, "h.jpg")
	write(t, filepath.Join(root, "a.jpg"), "a")
	write(t, hidden, "h")
	scan(t, root)
	guid := itemAt(t, hidden).Guid

	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	scan(t, root)
	if item := itemAt(t, hidden); item.Guid != guid {
		t.Error("a file in an unreadable directory was taken as deleted")
	}
}

func TestValidatorMissingRootDeletesNothing(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "library")
	a := filepath.Join(root, "a.jpg")
	write(t, a, "a")
	scan(t, root)
	guid := itemAt(t, a).Guid

	// The drive is unmounted: the root is gone
	if err := os.Rename(root, filepath.Join(parent, "elsewhere")); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	if _, err := testDB.GetItemByGuid(guid); err != nil {
		t.Error("a missing root deleted the library")
	}
}

// Removed in one walk (the item is deleted), back at another path in a later one:
// the same item returns
func TestValidatorDeletedThenBack(t *testing.T) {
	root, away := t.TempDir(), t.TempDir()
	a := filepath.Join(root, "a.jpg")
	write(t, a, "coming back")
	scan(t, root)
	guid := itemAt(t, a).Guid

	parked := filepath.Join(away, "a.jpg")
	if err := os.Rename(a, parked); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "keep.jpg"), "keeps the root non-empty")
	scan(t, root)
	assertNoItem(t, guid)

	back := filepath.Join(root, "back", "a.jpg")
	if err := os.MkdirAll(filepath.Dir(back), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parked, back); err != nil {
		t.Fatal(err)
	}
	scan(t, root)
	if item := itemAt(t, back); item.Guid != guid || item.State != dto.Visible {
		t.Errorf("back: %+v, want GUID %s, Visible", item, guid)
	}
}

// A broken file (exiftool's Error, or an image with no size) is not an item and is
// not processed again on the next walk; fixed, it becomes one. A photo that gets
// corrupted is no longer shown.
func TestValidatorBrokenFiles(t *testing.T) {
	root := t.TempDir()
	corrupt := filepath.Join(root, "corrupt.jpg")
	cut := filepath.Join(root, "cut.jpg")
	good := filepath.Join(root, "good.jpg")
	write(t, corrupt, "BROKEN garbage")
	write(t, cut, "NOSIZE header only")
	write(t, good, "a good photo")

	processed := scan(t, root)
	if len(processed) != 1 || processed[0] != good {
		t.Fatalf("first walk processed %v, want only the good one", processed)
	}
	for _, p := range []string{corrupt, cut} {
		if f, err := testDB.GetFileByPath(p); err != nil || !f.IsIgnored() {
			t.Errorf("%s: not ignored (%v)", filepath.Base(p), err)
		}
	}
	if again := scan(t, root); len(again) != 0 {
		t.Errorf("second walk processed %v, want nothing (broken ones wait for a change)", again)
	}

	// Fixed: processed again, an item now
	write(t, corrupt, "a repaired photo, longer")
	if fixed := scan(t, root); len(fixed) != 1 || fixed[0] != corrupt {
		t.Errorf("after the fix processed %v, want the repaired one", fixed)
	}
	itemAt(t, corrupt)

	// Corrupted later: its item goes
	guid := itemAt(t, good).Guid
	write(t, good, "BROKEN now, and longer than before")
	scan(t, root)
	assertNoItem(t, guid)
}
