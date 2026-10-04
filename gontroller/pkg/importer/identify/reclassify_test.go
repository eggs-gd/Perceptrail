package identify

import (
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

var testDB interface {
	model.ItemsApi
	model.FilesApi
	model.MetaApi
} // the whole DB: the tests set up what the steps only see a part of

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "identify-test")
	if err := model.Configure(model.DBConfig{Driver: model.DriverSQLite, Name: filepath.Join(dir, "t.db")}); err != nil {
		panic(err)
	}
	testDB = model.NewProxy(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// A new MIME detection classifies the ignored groups once more: their files are
// not ignored any more (the gate lets them through on the next walk)
func TestReclassifyIgnored(t *testing.T) {
	clip := filepath.Join(t.TempDir(), "clip.mov")
	if err := os.WriteFile(clip, []byte("a video the old detection missed"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(clip)
	f, err := testDB.CreateFile(dto.ItemEntry{Path: clip, Name: "clip.mov", Size: info.Size(), ModTime: info.ModTime()})
	if err != nil {
		t.Fatal(err)
	}
	f.SetIgnored()
	if _, err := testDB.UpdateFile(f); err != nil {
		t.Fatal(err)
	}
	if err := testDB.SetMeta(mimeVersionKey, "1"); err != nil {
		t.Fatal(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	if err := reclassifyIgnored(testDB, logger); err != nil {
		t.Fatal(err)
	}
	if v, _ := testDB.GetMeta(mimeVersionKey); v != mimeVersion {
		t.Errorf("version %q, want %q", v, mimeVersion)
	}
	if f, _ := testDB.GetFileByPath(clip); f.IsIgnored() {
		t.Error("the file is still ignored")
	}
}
