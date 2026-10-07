// The render service over a real model and real files: an item published wakes it,
// its rendition lands in the cache, the queue says it is done; a file that cannot
// be rendered is kept as a failure, not retried at once
package render_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/render"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
)

// start: a model and a running render service over a temporary data dir
func start(t *testing.T) (*model.Proxy, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	conf := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(conf, []byte("render:\n  enabled: true\n  workers: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(conf)
	if err != nil {
		t.Fatal(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	db, err := model.Open(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { render.New(cfg, db, logger).Start(ctx); close(stopped) }()
	t.Cleanup(func() { stop(); <-stopped; db.Close() })
	return db, cfg
}

// publish: a plain folder's item of this file, through the cheap stage
func publish(t *testing.T, db *model.Proxy, path string) *dto.ItemDto {
	t.Helper()
	f, err := db.CreateFile(dto.ItemEntry{Path: path, Name: filepath.Base(path)})
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateItem(f)
	if err != nil {
		t.Fatal(err)
	}
	item.HashShort, item.PreviewPath = "fingerprint", path
	if _, err := db.Publish(item); err != nil {
		t.Fatal(err)
	}
	return item
}

// eventually: the condition within a few seconds (render runs on its own)
func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("not within 5 s: %s", what)
}

func due(t *testing.T, db *model.Proxy) int {
	t.Helper()
	items, _, err := db.Due("render", "stand-in-1", dto.Cursor{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	return len(items)
}

func TestRender(t *testing.T) {
	db, cfg := start(t)
	src := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(src, []byte("not really a jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := publish(t, db, src)

	rendition := filepath.Join(cfg.CacheDir(), "r", item.GUID.String(), "stand-in-1-0.jpg")
	eventually(t, "the rendition in the cache", func() bool {
		_, err := os.Stat(rendition)
		return err == nil
	})
	eventually(t, "the queue says done", func() bool {
		row, err := db.Work(item.GUID, "render")
		return err == nil && row.DoneAt != 0
	})
	if n := due(t, db); n != 0 {
		t.Errorf("done and still due: %d", n)
	}
	if got, err := os.ReadFile(rendition); err != nil || string(got) != "not really a jpeg" {
		t.Errorf("rendition %q, %v", got, err)
	}
}

// A file gone before its render: a failure, backed off — not due again at once
func TestRenderFails(t *testing.T) {
	db, _ := start(t)
	item := publish(t, db, filepath.Join(t.TempDir(), "gone.jpg"))
	eventually(t, "the failure kept", func() bool {
		row, err := db.Work(item.GUID, "render")
		return err == nil && row.Error != ""
	})
	if row, _ := db.Work(item.GUID, "render"); row.Attempts != 1 || row.DoneAt != 0 || row.LeaseUntil != 0 {
		t.Errorf("row %+v: one attempt, not done, no lease", row)
	}
	if n := due(t, db); n != 0 {
		t.Errorf("due again right after its failure: %d", n)
	}
}
