package importer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/importer/commit"
	"perceptrail/gontroller/pkg/importer/gate"
	"perceptrail/gontroller/pkg/importer/group"
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/importer/walk"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/plugins/exif_date"
	"perceptrail/gontroller/pkg/plugins/exif_size"
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/providers/apple"
	"perceptrail/gontroller/pkg/providers/folder"
	"testing"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
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
	// The registry as the server loads it: the built-in perceptors (their tags are what
	// identify reads)
	if err := plugins.Load(plugins.Config{DataDir: dir, Logger: logger}, exif_date.Perceptor, exif_size.Perceptor); err != nil {
		panic(err)
	}

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

// harness: the import's steps as the chain has them, run one group at a time — the
// providers (Apple, the plain folder last), the gate, identify's steps (a fake
// exiftool), the core perceptors, close; after a walk its flush (every grouper gives
// what it holds) and the cycle's deletions
type harness struct {
	t      *testing.T
	logger *l.Logger
	ps     []providers.Provider
	sw     group.Switch
	gate   *gate.Gate
	// identify's own chain, driven a group at a time (Send, Flush)
	stored     *chain.Pipe[gate.Group]
	identified chan *identify.Item
	flushed    chan struct{}
	errs       chan error
	// the main files that reached the perceptors
	processed []string
}

func newHarness(t *testing.T) *harness {
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	ps := []providers.Provider{apple.New("", nil, itemsProxy, logger), folder.New()}
	h := &harness{t: t, logger: logger, ps: ps, sw: group.Switch{Providers: ps}, gate: gate.NewGate(testDB, logger),
		stored: chain.NewPipe[gate.Group](0), identified: make(chan *identify.Item, 1),
		flushed: make(chan struct{}, 1), errs: make(chan error, 10)}
	out := chain.NewPipe[*identify.Item](0)
	c := chain.New(h.errs)
	c.AddStep(identify.New(testDB, fakeTool{}, t.TempDir(), logger, h.stored, out))
	c.AddStep(chain.Sink(out, func(it *identify.Item) { h.identified <- it }, func() { h.flushed <- struct{}{} }))
	go c.Process(t.Context())
	return h
}

// identify: one group through identify's chain — what came out before its flush
// (nothing: the group was skipped)
func (h *harness) identify(g gate.Group) (*identify.Item, bool) {
	h.stored.Send(h.t.Context(), g)
	h.stored.Flush(h.t.Context())
	var it *identify.Item
	select {
	case it = <-h.identified:
		<-h.flushed
	case <-h.flushed:
	}
	select {
	case err := <-h.errs:
		h.t.Fatal(err)
	default:
	}
	return it, it != nil
}

// fakeTool stands in for exiftool: a file's content is its metadata (fakeExif); no
// embedded previews
type fakeTool struct{}

func (fakeTool) Read(paths, _ []string) ([]api.RawExif, error) {
	out := make([]api.RawExif, len(paths))
	for i, p := range paths {
		if m, err := fakeExif(p); err == nil {
			out[i] = m
		}
	}
	return out, nil
}

func (fakeTool) Extract(string, string, string) error { return errors.New("no exiftool in tests") }

func (h *harness) ok(err error) bool {
	if errors.Is(err, chain.ErrSkippedItem) {
		return false
	}
	if err != nil {
		h.t.Fatal(err)
	}
	return true
}

// toGate: one group from the gate on, through every step
func (h *harness) toGate(g providers.Group) {
	stored, err := h.gate.Decorate(g)
	if !h.ok(err) {
		return
	}
	it, ok := h.identify(stored)
	if !ok {
		return
	}
	runCorePlugins(h.t, it)
	if _, err := commit.NewCloser(itemsProxy).Decorate(it); err != nil {
		h.t.Fatal(err)
	}
	h.processed = append(h.processed, it.Item.Path)
}

// scan: one walk of root; the main files processed in it
func (h *harness) scan(root string) []string {
	h.processed = nil
	r := walk.Once(h.t.Context(), root, h.logger, func(e dto.ItemEntry) {
		i, err := h.sw.Route(e)
		if !h.ok(err) {
			return
		}
		if g, err := h.ps[i].Grouper().Decorate(e); h.ok(err) {
			h.toGate(g)
		}
	})
	// The walk's flush: every grouper gives what it holds; then the cycle's deletions
	for _, p := range h.ps {
		held, err := p.Grouper().(chain.Flusher[providers.Group]).Flush()
		if err != nil {
			h.t.Fatal(err)
		}
		for _, g := range held {
			h.toGate(g)
		}
	}
	deleteGone(testDB, r, h.logger)
	return h.processed
}

// refresh: one asset asked again on demand, as Refresh sends it
func (h *harness) refresh(key string) []string {
	h.processed = nil
	for _, p := range h.ps {
		if a, ok := p.Regroup(key); ok {
			h.toGate(providers.Group{Asset: a, Requested: true})
		}
	}
	return h.processed
}

// scan runs one walk through the steps of the import chain (a new harness: the
// groupers start empty, as on a restart)
func scan(t *testing.T, root string) []string {
	t.Helper()
	return newHarness(t).scan(root)
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
func runCorePlugins(t *testing.T, it *identify.Item) {
	t.Helper()
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	for _, p := range []api.Perceptor{exif_date.Perceptor, exif_size.Perceptor} {
		if _, err := p.(plugins.ExifCorePerceptor).Decorator(logger).Decorate(it); err != nil {
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
