package identify_test

import (
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/internal/importer/identify"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// At start, what changed in identify since the last run (the versions kept in the
// meta table): a new kinds' table makes the ignored files judged again; a new
// fingerprint makes every item forget its old one. The same versions change nothing.
func TestMigrate(t *testing.T) {
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	clip := filepath.Join(t.TempDir(), "clip.mov")
	if err := os.WriteFile(clip, []byte("a video the old detection missed"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(clip)
	ignored, err := testDB.CreateFile(dto.ItemEntry{Path: clip, Name: "clip.mov", Size: info.Size(), ModTime: info.ModTime()})
	if err != nil {
		t.Fatal(err)
	}
	ignored.SetIgnored()
	if _, err := testDB.UpdateFile(ignored); err != nil {
		t.Fatal(err)
	}
	old, err := testDB.CreateFile(dto.ItemEntry{Path: filepath.Join(t.TempDir(), "old.jpg"), Name: "old.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := testDB.CreateItem(old)
	if err != nil {
		t.Fatal(err)
	}
	item.HashShort = "exif-hash"
	if _, err := testDB.UpdateItem(item); err != nil {
		t.Fatal(err)
	}
	// The versions an older run kept (the meta table's keys are the DB's contract)
	for key := range map[string]bool{"mime_version": true, "hash_version": true} {
		if err := testDB.SetMeta(key, "1"); err != nil {
			t.Fatal(err)
		}
	}

	identify.Migrate(testDB, logger)
	if f, _ := testDB.GetFileByPath(clip); f.IsIgnored() {
		t.Error("the file is still ignored")
	}
	if got, _ := testDB.GetItemByGuid(item.Guid); got.HashShort != "" {
		t.Errorf("fingerprint kept: %q", got.HashShort)
	}
	for _, key := range []string{"mime_version", "hash_version"} {
		if v, _ := testDB.GetMeta(key); v == "1" {
			t.Errorf("%s still %q", key, v)
		}
	}

	// The same versions: nothing changes
	item.HashShort = "new-hash"
	if _, err := testDB.UpdateItem(item); err != nil {
		t.Fatal(err)
	}
	identify.Migrate(testDB, logger)
	if got, _ := testDB.GetItemByGuid(item.Guid); got.HashShort != "new-hash" {
		t.Errorf("the same version cleared it: %q", got.HashShort)
	}
}
