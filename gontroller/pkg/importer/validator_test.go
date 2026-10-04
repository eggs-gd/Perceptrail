package importer

import (
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/library"
	"perceptrail/gontroller/pkg/library/apple"
	"perceptrail/gontroller/pkg/library/folder"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/perceptor"
	"perceptrail/gontroller/pkg/perceptor/date"
	"perceptrail/gontroller/pkg/perceptor/size"
	"slices"
	"testing"
	"time"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// One sqlite database for the package (model keeps a single connection); every test
// uses its own library root, and deletions are scoped to the root.
// The DB the tests read and write (the stages get it in their constructors)
var (
	testDB interface {
		model.ItemsApi
		model.FilesApi
		model.MetaApi
	} // the whole DB: the tests set up what the steps only see a part of
	filesProxy model.FilesApi
	itemsProxy model.ItemsApi
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gontroller-scan-test")
	if err != nil {
		panic(err)
	}
	if err := model.Configure(model.DBConfig{Driver: model.DriverSQLite, Name: filepath.Join(dir, "test.db")}); err != nil {
		panic(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	testDB = model.NewProxy(logger)
	filesProxy, itemsProxy = testDB, testDB
	// The registry and the providers as the server loads them: the built-in
	// perceptors (their tags are what identify reads); Apple, the plain folder last
	if err := perceptor.Load(perceptor.Config{DataDir: dir, Logger: logger}, date.Perceptor, size.Perceptor); err != nil {
		panic(err)
	}
	library.Enable(apple.New("", nil, itemsProxy, logger), folder.New())

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// scan runs one pass of the import as the service builds it (importChain) over
// root, with a real exiftool; the main files of the items published in it. A new
// chain each time: the groupers start empty, as on a restart.
func scan(t *testing.T, root string) []string {
	t.Helper()
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	s := &importerService{db: testDB, logger: logger, errch: make(chan error, 10), root: root, cacheDir: t.TempDir()}
	start := time.Now()
	s.importChain().Process(t.Context())
	select {
	case err := <-s.errch:
		t.Fatal(err)
	default:
	}
	var published []string
	err := itemsProxy.StreamItemsSince(start.Add(-time.Second), func(it *dto.ItemDto, _ []*dto.FileDto) error {
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

func itemAt(t *testing.T, path string) *dto.ItemDto {
	t.Helper()
	item, err := itemsProxy.GetItemByPath(path)
	if err != nil {
		t.Fatalf("no item at %s: %v", path, err)
	}
	return item
}

func assertNoItem(t *testing.T, guid string) {
	t.Helper()
	if _, err := itemsProxy.GetItemByGuid(guid); err == nil {
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
	if f, err := filesProxy.GetFileByPath(moved); err != nil || f.GUID != guidB || f.LinkedTo != guidB {
		t.Errorf("moved: file row %+v, %v", f, err)
	}
	if _, err := filesProxy.GetFileByPath(b); err == nil {
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
	if _, err := filesProxy.GetFileByPath(dup); err == nil {
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
	if _, err := itemsProxy.GetItemByGuid(guid); err != nil {
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
		if f, err := filesProxy.GetFileByPath(p); err != nil || !f.IsIgnored() {
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
