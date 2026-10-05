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

// Many writers at once, a failing rule among them (an error, a panic): it rolls
// back alone, everything else is written
func TestFailingRuleAlone(t *testing.T) {
	db := openTest(t)
	var writers sync.WaitGroup
	for i := range 50 {
		writers.Go(func() {
			key := fmt.Sprint("k", i)
			switch i {
			case 7:
				_, err := testRule(db, func(q *Proxy) error {
					q.setMeta(key, "half")
					return errors.New("a rule gives up")
				})
				if err == nil {
					t.Error("a failing rule returned nil")
				}
			case 13:
				_, err := testRule(db, func(q *Proxy) error {
					q.setMeta(key, "half")
					panic("a rule breaks")
				})
				if err == nil {
					t.Error("a panicking rule returned nil")
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

// A rule calls another rule directly — same transaction, rolled back with it; a
// public write inside a rule is a mistake (the writer would wait for itself): an
// error, not a hang
func TestRuleInsideRule(t *testing.T) {
	db := openTest(t)
	_, err := testRule(db, func(q *Proxy) error {
		if err := q.setMeta("inner", "x"); err != nil {
			return err
		}
		return errors.New("the rule fails after it")
	})
	if err == nil {
		t.Fatal("the rule's error got lost")
	}
	if v, _ := db.GetMeta("inner"); v != "" {
		t.Errorf("the inner write outlived its rule: %q", v)
	}
	if _, err := testRule(db, func(q *Proxy) error { return q.SetMeta("public", "x") }); err == nil {
		t.Error("a public write inside a rule did not fail")
	}
}

// Asynchronous: submitted to a rule's topic, the result comes by subscription,
// after the commit
func TestTopic(t *testing.T) {
	db := openTest(t)
	created := db.CreateFilesTopic()
	results := created.Subscribe(4)
	defer results.Close()
	id := created.Submit([]dto.ItemEntry{{Path: "/a.jpg", Name: "a.jpg"}, {Path: "/b.jpg", Name: "b.jpg"}})
	r := <-results.C
	if r.ID != id || r.Err != nil || len(r.Value) != 2 || r.Value[0].ID == 0 {
		t.Fatalf("result %+v", r)
	}
	if f, err := db.FindFile("/b.jpg"); err != nil || f == nil {
		t.Errorf("not in the database after its result: %v, %v", f, err)
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

// testRule: a rule of the test's own, run as a write
func testRule(db *Proxy, fn func(q *Proxy) error) (struct{}, error) {
	return rule(db, pubsub.Frame, func(q *Proxy, _ struct{}) (struct{}, error) { return struct{}{}, fn(q) }).Do(struct{}{})
}
