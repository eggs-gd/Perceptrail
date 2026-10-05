package model

import (
	"errors"
	pubsub "github.com/eggs-gd/go-pub-sub"

	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
)

// An item's life: the rules about the library's data — when a group needs work,
// what a file gone means, which item a group is, when an item is shown. Every chain
// that touches items (the import, an asset again on demand, later the API
// providers and the maintenance) goes by the same ones; a step gathers the facts
// (stat, exif, kinds, fingerprint), the model decides and keeps.

// NeedsWork: the group's files are stored and unchanged on disk — does it still
// need work? Yes when a file was never linked or is linked outside the group (its
// main file is gone: a RAW deleted, its JPEG left), a file has no role (from before
// roles existed), the item is missing or not through the cheap stage, it has no
// fingerprint (the fingerprint changed: it gets the new one), or the source's
// metadata changed while the files did not (metaHash), or it is marked for rework
// (MarkRework). guid: the group's item; "" when the whole group is ignored (not media,
// broken).
func (q query) NeedsWork(files []*dto.FileDto, key, metaHash string) (needs bool, guid string, err error) {
	inGroup := map[string]bool{key: key != ""}
	for _, f := range files {
		inGroup[f.GUID] = true
	}
	for _, f := range files {
		switch {
		case f.LinkedTo == "":
			return true, "", nil
		case !f.IsIgnored() && f.Role == "":
			return true, "", nil
		case f.IsIgnored():
		case !inGroup[f.LinkedTo]:
			return true, "", nil
		case guid == "":
			guid = f.LinkedTo
		}
	}
	if guid == "" {
		return false, "", nil
	}
	item, err := q.GetItemByGuid(guid)
	if errors.Is(err, ErrNotFound) {
		return true, guid, nil
	}
	if err != nil {
		return false, guid, err
	}
	return !cheapStageDone(item) || item.HashShort == "" || item.MetaHash != metaHash || item.Rework, guid, nil
}

// cheapStageDone: the item went through the cheap stage (Visible, Waiting) or is
// fully done (Ready). An item shown without a preview is from before the cheap
// stage existed: it goes through once more.
func cheapStageDone(item *dto.ItemDto) bool {
	switch item.State {
	case dto.Visible, dto.Ready:
		return item.PreviewPath != ""
	case dto.Waiting:
		return true
	}
	return false
}

// gone: these files are gone for the library (the walk found them missing, or
// their provider says so).
// A main file gone: its item is deleted; a sidecar gone: its item is Dirty (processed
// again); an item with no files left is gone too (a keyed asset: every file is
// "linked", none is "main" by its own GUID). The files' rows go.
func (t *tx) gone(files []*dto.FileDto) (GoneResult, error) {
	var deleted, dirty int
	for _, f := range files {
		switch {
		case f.IsIgnored() || f.LinkedTo == "":
		case f.LinkedTo == f.GUID: // main file: the item is gone
			item, err := t.GetItemByGuid(f.GUID)
			if err != nil {
				continue // never became an item, or already deleted
			}
			if _, err := t.deleteItem(item); err != nil {
				return GoneResult{deleted, dirty}, err
			}
			deleted++
		default: // sidecar: its item must be processed again
			item, err := t.GetItemByGuid(f.LinkedTo)
			if err != nil {
				continue
			}
			item.State = dto.Dirty
			if _, err := t.updateItem(item); err != nil {
				return GoneResult{deleted, dirty}, err
			}
			dirty++
		}
	}
	if _, err := t.deleteFiles(files); err != nil {
		return GoneResult{deleted, dirty}, err
	}
	for _, f := range files {
		if f.LinkedTo == "" || f.IsIgnored() {
			continue
		}
		if n, err := t.CountLinkedFiles(f.LinkedTo); err == nil && n == 0 {
			if item, err := t.GetItemByGuid(f.LinkedTo); err == nil {
				if _, err := t.deleteItem(item); err == nil {
					deleted++
				}
			}
		}
	}
	return GoneResult{deleted, dirty}, nil
}

// ignore: the group is not an item (not media, or its main file is broken): its
// files are remembered as ignored — the gate skips them until a file changes — and
// an item its main file used to be (it got corrupted) goes
func (t *tx) ignore(files []*dto.FileDto) (pubsub.None, error) {
	if item, err := t.GetItemByGuid(files[0].GUID); err == nil {
		if _, err := t.deleteItem(item); err != nil {
			return pubsub.None{}, err
		}
	}
	for _, f := range files {
		f.SetIgnored()
	}
	_, err := t.updateFiles(files)
	return pubsub.None{}, err
}

// validateGroup: the item of a plain folder's group (files: the main file first).
// Its files link to the main file; a file that was the main file of its own item
// before is a sidecar now (a JPEG imported alone, then its RAW appeared): that item
// goes. Then the main file's item by its path and fingerprint (ValidateFile).
func (t *tx) validateGroup(a ValidateGroupArgs) (*dto.ItemDto, error) {
	files, hash := a.Files, a.Hash
	main := files[0]
	for _, f := range files {
		f.LinkTo(main)
	}
	for _, f := range files[1:] {
		if old, err := t.GetItemByGuid(f.GUID); err == nil {
			if _, err := t.deleteItem(old); err != nil {
				return nil, err
			}
			t.logger.Info("Former main file is a sidecar now", l.String("file", f.Path), l.String("main", main.Path))
		}
	}
	if _, err := t.updateFiles(files); err != nil {
		return nil, err
	}
	return t.validateFile(main, hash)
}

// validateAsset: the item of a group whose source knows its identity (an Apple
// Photos asset UUID: the key, whatever the main file is). Every file links to it; an
// item of a file's own from before (the plain folder's grouper read the library's
// originals) goes. Then the keyed item (validateKeyed).
func (t *tx) validateAsset(a ValidateAssetArgs) (*dto.ItemDto, error) {
	key, files, hash := a.Key, a.Files, a.Hash
	for _, f := range files {
		if f.GUID != key {
			if old, err := t.GetItemByGuid(f.GUID); err == nil {
				if _, err := t.deleteItem(old); err != nil {
					return nil, err
				}
			}
		}
		f.LinkToItem(key)
	}
	if _, err := t.updateFiles(files); err != nil {
		return nil, err
	}
	return t.validateKeyed(key, files[0], hash)
}

// publish: the item at the end of the import's cheap stage — Visible when it has
// something the browser shows (a preview), else Waiting (the expensive stage later)
func (t *tx) publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	item.Rework = false
	item.State = dto.Waiting
	if item.PreviewPath != "" {
		item.State = dto.Visible
	}
	return t.updateItem(item)
}

// markRework: these items are processed again on the next walk (NeedsWork), their
// files unchanged — e.g. a perceptor has no row for them, a library made a file of
// one local; publishing clears the mark. Not a change the client sees: updated_at
// stays (the client's delta would bring the item back in its old state)
func (t *tx) markRework(guids []string) (int64, error) {
	var n int64
	for start := 0; start < len(guids); start += 500 { // under SQLite's variable limit
		page := guids[start:min(start+500, len(guids))]
		res := t.db.Model(&dto.ItemDto{}).Where("guid IN ?", page).UpdateColumn("rework", true)
		if res.Error != nil {
			return n, res.Error
		}
		n += res.RowsAffected
	}
	return n, nil
}

// Unshown: the items nothing can show yet (no file the browser shows, no preview:
// Waiting) — what a library that draws renditions itself may fill
func (q query) Unshown() ([]*dto.ItemDto, error) {
	return q.GetItemsInStates(dto.Waiting)
}
