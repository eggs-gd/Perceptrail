package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
)

func openTest(t *testing.T) *Proxy {
	t.Helper()
	db, err := Open(database{config.Database{Driver: config.DriverSQLite, Name: filepath.Join(t.TempDir(), "x.db")}},
		l.NewLogger(l.ErrorLevel, &tree.Decorator{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// A write's result comes after the commit: the readers see it at once
func TestWriteThenRead(t *testing.T) {
	db := openTest(t)
	if err := db.SetMeta("k", "v"); err != nil {
		t.Fatal(err)
	}
	if v, err := db.GetMeta("k"); err != nil || v != "v" {
		t.Errorf("read %q, %v", v, err)
	}
}

// Many writers at once, a failing write among them (an error, a panic): it rolls
// back alone, everything else is written
func TestFailingWriteAlone(t *testing.T) {
	db := openTest(t)
	var writers sync.WaitGroup
	for i := range 50 {
		writers.Go(func() {
			key := fmt.Sprint("k", i)
			switch i {
			case 7:
				_, err := testWrite(db, func(in *tx) error {
					in.setMeta(MetaArgs{key, "half"})
					return errors.New("a write gives up")
				})
				if err == nil {
					t.Error("a failing write returned nil")
				}
			case 13:
				_, err := testWrite(db, func(in *tx) error {
					in.setMeta(MetaArgs{key, "half"})
					panic("a write breaks")
				})
				if err == nil {
					t.Error("a panicking write returned nil")
				}
			default:
				if err := db.SetMeta(key, "ok"); err != nil {
					t.Error(err)
				}
			}
		})
	}
	writers.Wait()
	for i := range 50 {
		v, _ := db.GetMeta(fmt.Sprint("k", i))
		if (i == 7 || i == 13) != (v == "") {
			t.Errorf("k%d = %q", i, v)
		}
	}
}

// A write calls another write directly — same transaction, rolled back with it (a
// public write cannot be called there: tx has none)
func TestWriteInsideWrite(t *testing.T) {
	db := openTest(t)
	_, err := testWrite(db, func(in *tx) error {
		if _, err := in.setMeta(MetaArgs{"inner", "x"}); err != nil {
			return err
		}
		return errors.New("the write fails after it")
	})
	if err == nil {
		t.Fatal("the write's error got lost")
	}
	if v, _ := db.GetMeta("inner"); v != "" {
		t.Errorf("the inner write outlived its write: %q", v)
	}
}

// Asynchronous: submitted through a client, its own result comes back after the
// commit; Done's listeners hear it too
func TestCommand(t *testing.T) {
	db := openTest(t)
	created := db.CreateFilesCommand()
	var heard []pubsub.ID
	created.Done().Subscribe(func(r pubsub.Result[[]*dto.FileDto]) { heard = append(heard, r.ID) })
	files := created.Client(1)
	id := files.Submit([]dto.ItemEntry{{Path: "/a.jpg", Name: "a.jpg"}, {Path: "/b.jpg", Name: "b.jpg"}})
	files.Close()
	for r := range files.Results() {
		if r.ID != id || r.Err != nil || len(r.Value) != 2 || r.Value[0].ID == 0 {
			t.Fatalf("result %+v", r)
		}
		if f, err := db.FindFile("/b.jpg"); err != nil || f == nil {
			t.Errorf("not in the database after its result: %v, %v", f, err)
		}
	}
	if len(heard) != 1 || heard[0] != id {
		t.Errorf("Done heard %v, want [%d]", heard, id)
	}
}

// Close writes what is queued; a write after it fails instead of hanging
func TestClose(t *testing.T) {
	db := openTest(t)
	if err := db.SetMeta("before", "x"); err != nil {
		t.Fatal(err)
	}
	db.writer.close()
	if err := db.SetMeta("after", "x"); !errors.Is(err, errClosed) {
		t.Errorf("a write after close: %v", err)
	}
}

// Closed, the database leaves no journal behind: the writer's connection closes
// last (a read-only one closing last could not merge and remove it)
func TestCloseLeavesNoJournal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	db, err := Open(database{config.Database{Driver: config.DriverSQLite, Name: path}}, l.NewLogger(l.ErrorLevel, &tree.Decorator{}))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMeta("k", "v")
	db.GetMeta("k") // a reader opened too
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, j := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + j); err == nil {
			t.Errorf("%s left after Close", j)
		}
	}
}

// The writer's connection runs with synchronous=NORMAL (1)
func TestSynchronousNormal(t *testing.T) {
	db := openTest(t)
	var mode int
	if err := db.writes.Raw("PRAGMA synchronous").Scan(&mode).Error; err != nil || mode != 1 {
		t.Errorf("synchronous = %d, %v", mode, err)
	}
}

// testWrite: a write of the test's own, run as a command
func testWrite(db *Proxy, fn func(in *tx) error) (pubsub.None, error) {
	return command(db, pubsub.Frame, func(in *tx, _ pubsub.None) (pubsub.None, error) { return pubsub.None{}, fn(in) }).Do(pubsub.None{})
}

// A write without a result is a Message: Submit its argument, a result per ID with
// its error; without an argument, a Signal: Submit nothing
func TestShapedCommands(t *testing.T) {
	db := openTest(t)
	meta := db.SetMetaCommand().Client(1)
	id := meta.Submit(MetaArgs{"k", "v"})
	meta.Close()
	for r := range meta.Results() {
		if r.ID != id || r.Err != nil {
			t.Fatalf("result %+v", r)
		}
	}
	if v, _ := db.GetMeta("k"); v != "v" {
		t.Errorf("k = %q", v)
	}
	if n, err := db.UnignoreFilesCommand().Do(); n != 0 || err != nil {
		t.Errorf("unignore: %d, %v", n, err)
	}
}
