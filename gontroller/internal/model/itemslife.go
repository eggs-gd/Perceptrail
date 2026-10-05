package model

import (
	"errors"

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
func (p *Proxy) NeedsWork(files []*dto.FileDto, key, metaHash string) (needs bool, guid string, err error) {
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
	item, err := p.GetItemByGuid(guid)
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

// Gone: these files are gone for the library (the walk found them missing, or
// their provider says so).
// A main file gone: its item is deleted; a sidecar gone: its item is Dirty (processed
// again); an item with no files left is gone too (a keyed asset: every file is
// "linked", none is "main" by its own GUID). The files' rows go.
func (p *Proxy) Gone(files []*dto.FileDto) (deleted, dirty int, err error) {
	err = p.write(func(q *Proxy) error {
		for _, f := range files {
			switch {
			case f.IsIgnored() || f.LinkedTo == "":
			case f.LinkedTo == f.GUID: // main file: the item is gone
				item, err := q.GetItemByGuid(f.GUID)
				if err != nil {
					continue // never became an item, or already deleted
				}
				if err := q.DeleteItem(item); err != nil {
					return err
				}
				deleted++
			default: // sidecar: its item must be processed again
				item, err := q.GetItemByGuid(f.LinkedTo)
				if err != nil {
					continue
				}
				item.State = dto.Dirty
				if _, err := q.UpdateItem(item); err != nil {
					return err
				}
				dirty++
			}
		}
		if err := q.DeleteFiles(files); err != nil {
			return err
		}
		for _, f := range files {
			if f.LinkedTo == "" || f.IsIgnored() {
				continue
			}
			if n, err := q.CountLinkedFiles(f.LinkedTo); err == nil && n == 0 {
				if item, err := q.GetItemByGuid(f.LinkedTo); err == nil {
					if err := q.DeleteItem(item); err == nil {
						deleted++
					}
				}
			}
		}
		return nil
	})
	return deleted, dirty, err
}

// Ignore: the group is not an item (not media, or its main file is broken): its
// files are remembered as ignored — the gate skips them until a file changes — and
// an item its main file used to be (it got corrupted) goes
func (p *Proxy) Ignore(files []*dto.FileDto) error {
	return p.write(func(q *Proxy) error {
		if item, err := q.GetItemByGuid(files[0].GUID); err == nil {
			if err := q.DeleteItem(item); err != nil {
				return err
			}
		}
		for _, f := range files {
			f.SetIgnored()
		}
		_, err := q.UpdateFiles(files)
		return err
	})
}

// ValidateGroup: the item of a plain folder's group (files: the main file first).
// Its files link to the main file; a file that was the main file of its own item
// before is a sidecar now (a JPEG imported alone, then its RAW appeared): that item
// goes. Then the main file's item by its path and fingerprint (ValidateFile).
func (p *Proxy) ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) {
		main := files[0]
		for _, f := range files {
			f.LinkTo(main)
		}
		for _, f := range files[1:] {
			if old, err := q.GetItemByGuid(f.GUID); err == nil {
				if err := q.DeleteItem(old); err != nil {
					return nil, err
				}
				q.logger.Info("Former main file is a sidecar now", l.String("file", f.Path), l.String("main", main.Path))
			}
		}
		if _, err := q.UpdateFiles(files); err != nil {
			return nil, err
		}
		return q.validateFile(main, hash)
	})
}

// ValidateAsset: the item of a group whose source knows its identity (an Apple
// Photos asset UUID: the key, whatever the main file is). Every file links to it; an
// item of a file's own from before (the plain folder's grouper read the library's
// originals) goes. Then the keyed item (validateKeyed).
func (p *Proxy) ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) {
		for _, f := range files {
			if f.GUID != key {
				if old, err := q.GetItemByGuid(f.GUID); err == nil {
					if err := q.DeleteItem(old); err != nil {
						return nil, err
					}
				}
			}
			f.LinkToItem(key)
		}
		if _, err := q.UpdateFiles(files); err != nil {
			return nil, err
		}
		return q.validateKeyed(key, files[0], hash)
	})
}

// Publish: the item at the end of the import's cheap stage — Visible when it has
// something the browser shows (a preview), else Waiting (the expensive stage later)
func (p *Proxy) Publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) {
		item.Rework = false
		item.State = dto.Waiting
		if item.PreviewPath != "" {
			item.State = dto.Visible
		}
		return q.UpdateItem(item)
	})
}

// MarkRework: these items are processed again on the next walk (NeedsWork), their
// files unchanged — e.g. a perceptor has no row for them, a library made a file of
// one local; publishing clears the mark. Not a change the client sees: updated_at
// stays (the client's delta would bring the item back in its old state)
func (p *Proxy) MarkRework(guids []string) (int64, error) {
	return written(p, func(q *Proxy) (int64, error) {
		var n int64
		for start := 0; start < len(guids); start += 500 { // under SQLite's variable limit
			page := guids[start:min(start+500, len(guids))]
			res := q.db.Model(&dto.ItemDto{}).Where("guid IN ?", page).UpdateColumn("rework", true)
			if res.Error != nil {
				return n, res.Error
			}
			n += res.RowsAffected
		}
		return n, nil
	})
}

// Unshown: the items nothing can show yet (no file the browser shows, no preview:
// Waiting) — what a library that draws renditions itself may fill
func (p *Proxy) Unshown() ([]*dto.ItemDto, error) {
	return p.GetItemsInStates(dto.Waiting)
}
