package exif_test

import (
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/importer/exif"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/perceptor"
	"perceptrail/gontroller/test/plugin"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// At start the import perceptors' rows meet the items: an item a perceptor has no
// row for is processed again, a row of an item that is gone is dropped. The built-in
// perceptors keep nothing, so a real plugin that keeps data (exif_geo) is built and
// loaded, as the server does.
func TestReconcile(t *testing.T) {
	plugin.Supported(t)
	dir := t.TempDir()
	so := plugin.Build(t, "../../../../perceptors", "exif_geo", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("plugins:\n  - "+so+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	db, err := model.Open(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := perceptor.Load(cfg, logger); err != nil {
		t.Fatal(err)
	}
	defer perceptor.Close()
	geo, ok := perceptor.Store("exif_geo")
	if !ok {
		t.Fatal("exif_geo keeps no storage")
	}

	// Two items: one processed by geo, one not; and a row of an item that is gone
	item := func(path string) string {
		f, err := db.CreateFile(dto.ItemEntry{Path: filepath.Join(dir, path)})
		if err != nil {
			t.Fatal(err)
		}
		it, err := db.CreateItem(f)
		if err != nil {
			t.Fatal(err)
		}
		return it.Guid
	}
	done, fresh := item("done.jpg"), item("fresh.jpg")
	for _, guid := range []string{done, "gone-item"} {
		if err := geo.Save(guid, nil); err != nil {
			t.Fatal(err)
		}
	}

	if err := exif.Reconcile(db, logger); err != nil {
		t.Fatal(err)
	}
	rows, err := geo.Guids()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0] != done {
		t.Errorf("rows %v, want only the live item's %s", rows, done)
	}
	for guid, rework := range map[string]bool{done: false, fresh: true} {
		if it, err := db.GetItemByGuid(guid); err != nil || it.Rework != rework {
			t.Errorf("%s: rework %v, want %v (%v)", guid, it.Rework, rework, err)
		}
	}
}
