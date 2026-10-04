// The Apple library over a real model, with a fake Photos: its background work
package library_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/config"
	"perceptrail/gontroller/pkg/library/apple"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/test/fake"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

var testDB *model.Proxy

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "library-test")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), nil, 0o644); err != nil {
		panic(err)
	}
	cfg, err := config.Read(filepath.Join(dir, "config.yml")) // the database in dir
	if err != nil {
		panic(err)
	}
	if testDB, err = model.Open(cfg, l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// Nothing local at all (Waiting): never on the sheet, so asked for in the
// background — once per run, only Photos assets; the item processed again after
func TestHydrateWaiting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Photos Library.photoslibrary")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	photos := &fake.Photos{Root: root}
	p := apple.New(filepath.Dir(root), photos, testDB, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))
	for _, it := range []*dto.ItemDto{
		{Guid: "H1111111-WAITING", State: dto.Waiting, Kind: dto.KindPhoto, Path: filepath.Join(root, "originals/H/H1111111-WAITING.heic")},
		{Guid: "H2222222-SHOWN", State: dto.Visible, Kind: dto.KindPhoto, Path: filepath.Join(root, "originals/H/H2222222-SHOWN.heic")},
		{Guid: "H3333333-FOLDER", State: dto.Waiting, Kind: dto.KindPhoto, Path: "/photos/h3.heic"},
	} {
		if _, err := testDB.UpdateItem(it); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	marked := func(guid string) bool {
		it, err := testDB.GetItemByGuid(guid)
		return err == nil && it.Rework
	}
	for deadline := time.Now().Add(5 * time.Second); !marked("H1111111-WAITING"); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the waiting asset was not asked for (not marked for the next walk)")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "resources/derivatives/H/H1111111-WAITING_1_102_o.jpeg")); err != nil {
		t.Error("the image is not in the library")
	}
	time.Sleep(200 * time.Millisecond)
	if marked("H2222222-SHOWN") || marked("H3333333-FOLDER") {
		t.Error("marked more than the waiting Photos asset")
	}
}
