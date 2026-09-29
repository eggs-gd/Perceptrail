package scan

import (
	"errors"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"testing"
	"time"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// One sqlite database for the package (model keeps a single connection); every test
// uses its own library root, and deletions are scoped to the root.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gontroller-scan-test")
	if err != nil {
		panic(err)
	}
	if err := model.Configure(model.DBConfig{Driver: model.DriverSQLite, Name: filepath.Join(dir, "test.db")}); err != nil {
		panic(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	filesProxy = model.NewProxy(logger)
	itemsProxy = model.NewProxy(logger)

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// scan runs the walker and the validator the way the import chain does, without
// exiftool: the file content stands in for its metadata. Returns the paths of the
// main files that were (re)processed.
func scan(t *testing.T, root string) []string {
	t.Helper()
	m := newTestMonitor(t, root)
	m.currentRun = time.Now()

	ch := make(chan inType)
	res := make(chan walkResult)
	go func() {
		r := m.walk(ch)
		close(ch)
		res <- r
	}()

	var processed []string
	process := func(group []*dto.FileDto, err error) {
		if errors.Is(err, chain.ErrSkippedItem) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(group[0].Path)
		if err != nil {
			t.Fatal(err)
		}
		item, err := itemsProxy.ValidateFile(group[0], api.RawExif{"Content": content})
		if err != nil {
			t.Fatal(err)
		}
		item.State = dto.Ready // the closer
		if _, err := itemsProxy.UpdateItem(item); err != nil {
			t.Fatal(err)
		}
		processed = append(processed, group[0].Path)
	}

	for in := range ch {
		process(m.Decorate(in))
	}
	result := <-res
	process(m.Decorate(inType{done: &result}))
	return processed
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if item := itemAt(t, a); item.Guid != guidA || item.HashShort == hashA || item.State != dto.Ready {
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
	scan(t, root)
	if item := itemAt(t, photo); item.Guid != guid || item.State != dto.Dirty {
		t.Errorf("sidecar gone: %+v, want the same item, Dirty", item)
	}

	// Dirty is processed on the next walk although no file changed
	if got := scan(t, root); len(got) != 1 || got[0] != photo {
		t.Errorf("dirty: processed %v", got)
	}
	if item := itemAt(t, photo); item.State != dto.Ready {
		t.Errorf("dirty: state %d after processing", item.State)
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
	if item := itemAt(t, back); item.Guid != guid || item.State != dto.Ready {
		t.Errorf("back: %+v, want GUID %s, Ready", item, guid)
	}
}
