// identify over a real model and a real exiftool, as the import runs it: a group in,
// the identified item out
package identify_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/config"
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/perceptor"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// One sqlite database for the package (the model keeps a single connection)
var testDB *model.Proxy

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "identify-test")
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
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	if testDB, err = model.Open(cfg, logger); err != nil {
		panic(err)
	}
	// The tags identify reads are what the loaded perceptors declare
	if err := perceptor.Load(cfg, logger); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// identifyGroup: the files (their rows, as the walk writes them) as one group through
// identify, the way the import runs it; the item that came out, nil if none
func identifyGroup(t *testing.T, cacheDir string, paths ...string) *identify.Item {
	t.Helper()
	group := dto.Asset{}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		row, err := testDB.CreateFile(dto.ItemEntry{Path: p, Name: filepath.Base(p), Size: info.Size(), ModTime: info.ModTime()})
		if err != nil {
			t.Fatal(err)
		}
		group.Files = append(group.Files, row)
	}
	in, out := make(chan dto.Asset), make(chan *identify.Item)
	got := &items{}
	errs := make(chan error, 10)
	c := chain.NewChainProcessor(errs)
	c.AddStep(chain.NewEntryPoint(in, one{group}))
	c.AddStep(identify.New(testDB, cacheDir, "exiftool", l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}), in, out))
	c.AddStep(chain.NewEnd(out, got))
	c.Process(context.Background())
	select {
	case err := <-errs:
		t.Fatal(err)
	default:
	}
	if len(got.items) == 0 {
		return nil
	}
	return got.items[0]
}

// one: an entry point of one group
type one struct{ group dto.Asset }

func (o one) Start(_ context.Context, emit func(dto.Asset) bool) error {
	emit(o.group)
	return nil
}

// items: the end of the test chain
type items struct{ items []*identify.Item }

func (i *items) Consume(it *identify.Item) error {
	i.items = append(i.items, it)
	return nil
}
