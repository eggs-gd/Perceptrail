package model

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"perceptrail/gontroller/internal/model/dto"

	"gorm.io/gorm"
)

// validateFile: the item of a plain folder's main file, by its path and its
// fingerprint (hashShort: the file's bytes, see identify's fingerprint) — the same,
// moved, a new one or changed (Walker.puml)
func (t *tx) validateFile(item *dto.FileDto, hashShort string) (*dto.ItemDto, error) {
	itemByGUID, itemByPath, itemByHash := t.getItemsForValidation(item, hashShort)

	if itemByGUID.Guid != itemByPath.Guid {
		// probably panic(). Path/Guid should be stable pair on files layer
		return &dto.ItemDto{}, fmt.Errorf("there is path/guid missmatch")
	}

	if itemByGUID.Guid == "" {
		// Walker.puml "found moved": same hash, the old path no longer exists
		if moved := t.findMovedItem(item, hashShort); moved != nil {
			return moved, t.moveItem(moved, item)
		}

		// Not found, or a duplicate (same hash, the old path still exists): a new item.
		// Reusing a duplicate's thumbnails is a later optimisation.
		created, err := t.createItem(item)
		if err != nil {
			return created, err
		}
		created.HashShort = hashShort
		_, err = t.updateItem(created)
		return created, err
	}

	// The type may be known better now (mime step): keep the item's in sync
	if item.MimeType != "" && itemByGUID.MimeType != item.MimeType {
		itemByGUID.MimeType = item.MimeType
		if _, err := t.updateItem(itemByGUID); err != nil {
			return itemByGUID, err
		}
	}

	if itemByPath.Guid == itemByHash.Guid {
		// all three items are the same, known file
		return itemByGUID, nil
	}

	// Same path, new hash (a duplicate of another file with that hash or not):
	// modified, regenerate thumbs
	itemByGUID.State = dto.Dirty
	itemByGUID.HashShort = hashShort
	_, err := t.updateItem(itemByGUID)
	return itemByGUID, err
}

func (t *tx) getItemsForValidation(file *dto.FileDto, hashShort string) (byGuid *dto.ItemDto, byPath *dto.ItemDto, byHash *dto.ItemDto) {
	var itemByGUID, itemByPath *dto.ItemDto
	var itemsByHash []*dto.ItemDto
	var err error

	if itemByGUID, err = t.GetItemByGuid(file.GUID); err != nil {
		itemByGUID = &dto.ItemDto{}
	}

	if itemByPath, err = t.GetItemByPath(file.Path); err != nil {
		itemByPath = &dto.ItemDto{}
	}

	if itemsByHash, err = t.GetItemsByHash(hashShort); err != nil || len(itemsByHash) == 0 {
		return itemByGUID, itemByPath, &dto.ItemDto{}
	}

	if len(itemsByHash) == 1 {
		return itemByGUID, itemByPath, itemsByHash[0]
	}

	for _, item := range itemsByHash {
		// more than one hash match, return one with our guid
		if item.Guid == itemByGUID.Guid {
			return itemByGUID, itemByPath, item
		}
	}

	// more than one hash match but noone with our guid, counts as one more duplicate so return empty
	// will be threated as new item ig byGuid also empty
	// or modified if byGuid not empty
	return itemByGUID, itemByPath, &dto.ItemDto{}
}

// findMovedItem returns an item with the same content whose file is gone from its
// old path: file is that item, moved. Deleted items count too: the walker may
// finalize (delete the item of the vanished path) before the moved file reaches the
// validator — the import chain is asynchronous — or the file may come back later.
// Live items are preferred, then the most recently deleted.
func (t *tx) findMovedItem(file *dto.FileDto, hashShort string) *dto.ItemDto {
	var candidates []*dto.ItemDto
	err := t.db.Unscoped().Where("hash_short = ?", hashShort).
		Order("deleted_at IS NOT NULL, deleted_at DESC").Find(&candidates).Error
	if err != nil {
		return nil
	}
	for _, item := range candidates {
		if item.Path == file.Path {
			continue
		}
		if _, err := os.Stat(item.Path); errors.Is(err, fs.ErrNotExist) {
			return item
		}
	}
	return nil
}

// moveItem gives the item the new main file: the item keeps its GUID (thumbnails
// and client links are keyed by it), the new file rows take that GUID over. A
// deleted item is restored.
func (t *tx) moveItem(item *dto.ItemDto, file *dto.FileDto) error {
	oldGuid, newGuid := item.Guid, file.GUID
	fail := func(err error) error { return fmt.Errorf("move %s → %s: %w", oldGuid, file.Path, err) }

	// The old main row first: GUID is unique
	if err := t.db.Where("guid = ?", oldGuid).Delete(&dto.FileDto{}).Error; err != nil {
		return fail(err)
	}
	// Old sidecars that are gone as well; the ones still on disk stay linked
	var oldSidecars []dto.FileDto
	if err := t.db.Where("linked_to = ?", oldGuid).Find(&oldSidecars).Error; err != nil {
		return fail(err)
	}
	for _, f := range oldSidecars {
		if _, err := os.Stat(f.Path); errors.Is(err, fs.ErrNotExist) {
			if err := t.db.Delete(&f).Error; err != nil {
				return fail(err)
			}
		}
	}
	// The new group (main + sidecars) links to the item's GUID
	if err := t.db.Model(&dto.FileDto{}).Where("linked_to = ?", newGuid).Update("linked_to", oldGuid).Error; err != nil {
		return fail(err)
	}
	if err := t.db.Model(&dto.FileDto{}).Where("guid = ?", newGuid).Update("guid", oldGuid).Error; err != nil {
		return fail(err)
	}

	item.Path = file.Path
	item.MimeType = file.MimeType
	if item.DeletedAt.Valid { // deleted meanwhile: it is back
		item.DeletedAt = gorm.DeletedAt{}
		item.State = dto.Dirty
	}
	if err := t.db.Unscoped().Save(item).Error; err != nil {
		return fail(err)
	}
	file.GUID, file.LinkedTo = oldGuid, oldGuid
	return nil
}

// validateKeyed: the key is the item's identity (it never changes, whatever the
// main file is): same hash -> as is; another hash -> Dirty (the main file changed,
// e.g. a derivative replaced by the downloaded original); deleted -> restored.
func (t *tx) validateKeyed(key string, main *dto.FileDto, hash string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	err := t.db.Unscoped().Where("guid = ?", key).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = dto.ItemDto{Guid: key, State: dto.New, Path: main.Path, MimeType: main.MimeType, HashShort: hash}
		return &item, t.db.Create(&item).Error
	}
	if err != nil {
		return nil, err
	}
	if item.DeletedAt.Valid {
		item.DeletedAt = gorm.DeletedAt{}
		item.State = dto.Dirty
	}
	if item.HashShort != hash {
		item.HashShort = hash
		item.State = dto.Dirty
	}
	item.Path, item.MimeType = main.Path, main.MimeType
	return &item, t.db.Unscoped().Save(&item).Error
}
