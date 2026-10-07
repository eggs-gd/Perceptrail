package model

import (
	"errors"
	"testing"

	"perceptrail/gontroller/internal/model/dto"
)

// ItemPublished comes after the commit: a listener that reads the item through the
// readers' pool finds it published
func TestPublishedAfterCommit(t *testing.T) {
	db := openTest(t)
	f, err := db.CreateFile(dto.ItemEntry{Path: "/a.jpg", Name: "a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateItem(f)
	if err != nil {
		t.Fatal(err)
	}
	var heard []ItemPublished
	var seen dto.ItemState
	db.Published().Subscribe(func(e ItemPublished) {
		heard = append(heard, e)
		if it, err := db.GetItemByGUID(e.GUID); err == nil {
			seen = it.State
		}
	})
	if _, err := db.Publish(item); err != nil {
		t.Fatal(err)
	}
	if len(heard) != 1 || heard[0].GUID != item.GUID || heard[0].State != dto.Waiting {
		t.Fatalf("heard %+v", heard)
	}
	if seen != dto.Waiting {
		t.Errorf("the listener read state %v: the event came before the commit", seen)
	}
}

// A write rolled back to its savepoint takes its events with it; the batch's other
// writes keep theirs
func TestRolledBackWriteEmitsNothing(t *testing.T) {
	db := openTest(t)
	var heard []ItemPublished
	db.Published().Subscribe(func(e ItemPublished) { heard = append(heard, e) })

	_, err := testWrite(db, func(in *tx) error {
		emit(in, &in.events.published, ItemPublished{GUID: "gone"})
		return errors.New("the write fails after its event")
	})
	if err == nil {
		t.Fatal("the write's error got lost")
	}
	if _, err := testWrite(db, func(in *tx) error {
		emit(in, &in.events.published, ItemPublished{GUID: "kept"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(heard) != 1 || heard[0].GUID != "kept" {
		t.Errorf("heard %+v, want only the kept write's event", heard)
	}
}
