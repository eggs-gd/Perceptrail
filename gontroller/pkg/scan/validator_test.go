package scan

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins/exif_core"
	"perceptrail/gontroller/pkg/plugins/exif_core/date"
	"perceptrail/gontroller/pkg/plugins/exif_core/size"
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/providers/apple"
	"perceptrail/gontroller/pkg/providers/folder"
	"perceptrail/gontroller/pkg/scan/flow"
	"perceptrail/gontroller/pkg/scan/groups"
	"testing"

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

// fakeExif stands in for exiftool: the file content is its "metadata", so the
// short hash follows the content; no MIMEType, so mime uses the extension table.
// A size, like a real image's. "BROKEN…": exiftool's Error (a corrupt file);
// "NOSIZE…": a file with no image size (a JPEG cut after its header).
func fakeExif(path string) (api.RawExif, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch {
	case bytes.HasPrefix(content, []byte("BROKEN")):
		return api.RawExif{"Content": content, "Error": []byte("File format error")}, nil
	case bytes.HasPrefix(content, []byte("NOSIZE")):
		return api.RawExif{"Content": content}, nil
	}
	return api.RawExif{"Content": content, "ImageSize": []byte("4x3")}, nil
}

// scan runs one walk through the steps of the import chain in order, the way the
// chain wires them, with fakeExif. Returns the main files that reached the plugins.
func scan(t *testing.T, root string) []string {
	t.Helper()
	return scanWith(t, root, nil)
}

// scanWith: scan with the gate's dropped hook (nil: none)
func scanWith(t *testing.T, root string, dropped func(key string)) []string {
	t.Helper()
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	m := newTestMonitor(t, root)
	// The providers as the chain has them: Apple, the plain folder last
	ps := []providers.Provider{apple.New("", nil, itemsProxy, logger), folder.New()}
	sw := groups.Switch{Providers: ps}
	gate := newFilesGate(len(ps), newProgress(), dropped, logger)
	exif := &exifExtractor{logger: logger, extract: fakeExif}
	valid := newValidator(logger)
	preview := &cheapPreview{logger: logger, dir: t.TempDir(),
		extract: func(string, string, string) (string, error) { return "", errors.New("no exiftool in tests") }}

	var processed []string
	ok := func(err error) bool {
		if errors.Is(err, chain.ErrSkippedItem) {
			return false
		}
		if err != nil {
			t.Fatal(err)
		}
		return true
	}
	toGroupers := func(ev flow.FileEvent) {
		to, err := sw.Switch(ev)
		if !ok(err) {
			return
		}
		for i := range ps { // marker: every grouper, in order
			in, has := to[i]
			if !has {
				continue
			}
			group, err := ps[i].Grouper().Decorate(in)
			if !ok(err) {
				continue
			}
			{
				stored, err := gate.Decorate(group)
				if !ok(err) {
					continue
				}
				exifed, err := exif.Decorate(stored)
				if !ok(err) {
					continue
				}
				ranked, _ := mimeStep{}.Decorate(exifed)
				it, err := valid.Decorate(ranked)
				if !ok(err) {
					continue
				}
				it, _ = preview.Decorate(it)
				runCorePlugins(t, it)
				it.Item.State = dto.Waiting // the closer
				if it.Item.PreviewPath != "" {
					it.Item.State = dto.Visible
				}
				if _, err := itemsProxy.UpdateItem(it.Item); err != nil {
					t.Fatal(err)
				}
				processed = append(processed, it.Files[0].Path)
			}
		}
	}

	ch := make(chan inType)
	res := make(chan flow.WalkResult)
	go func() {
		r := m.walk(ch)
		close(ch)
		res <- r
	}()
	for in := range ch {
		ev, err := m.Decorate(in)
		if ok(err) {
			toGroupers(ev)
		}
	}
	result := <-res
	ev, _ := m.Decorate(inType{done: &result})
	toGroupers(ev)
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
	scan(t, root)
	if item := itemAt(t, photo); item.Guid != guid || item.State != dto.Dirty {
		t.Errorf("sidecar gone: %+v, want the same item, Dirty", item)
	}

	// Dirty is processed on the next walk although no file changed
	if got := scan(t, root); len(got) != 1 || got[0] != photo {
		t.Errorf("dirty: processed %v", got)
	}
	if item := itemAt(t, photo); item.State != dto.Visible {
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
	if item := itemAt(t, back); item.Guid != guid || item.State != dto.Visible {
		t.Errorf("back: %+v, want GUID %s, Visible", item, guid)
	}
}

// runCorePlugins passes the item through the core EXIF plugins (date, size) the
// way the plugin chain does
func runCorePlugins(t *testing.T, it *flow.RawItem) {
	t.Helper()
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, p := range []api.Perceptor{date.Perceptor, size.Perceptor} {
		in, out := make(chan exif_core.RawItemRW), make(chan exif_core.RawItemRW)
		proc := p.(exif_core.ExifCorePerceptor).NewProcessor(in, out, logger)
		errs := make(chan error, 1)
		chain.NewChainProcessor(errs).AddStep(proc)
		go proc.Process(ctx)
		in <- it
		select {
		case <-out:
		case err := <-errs:
			t.Fatal(err)
		}
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
