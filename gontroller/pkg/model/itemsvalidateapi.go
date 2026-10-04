package model

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"perceptrail/gontroller/pkg/model/dto"

	"gorm.io/gorm"
)

// Outcome of ValidateGroup, per Walker.puml
type Outcome int

const (
	OutcomeSame    Outcome = iota // same path, same hash
	OutcomeNew                    // not found, or a duplicate (same hash, the original still exists)
	OutcomeChanged                // same path, new hash
	OutcomeMoved                  // same hash, the old path is gone: the item keeps its GUID
)

// validateFile: the item of a plain folder's main file, by its path and its
// fingerprint (hashShort: the file's bytes, see identify's fingerprint)
func (p *Proxy) validateFile(item *dto.FileDto, hashShort string) (*dto.ItemDto, Outcome, error) {
	itemByGUID, itemByPath, itemByHash := p.getItemsForValidation(item, hashShort)

	if itemByGUID.Guid != itemByPath.Guid {
		// probably panic(). Path/Guid should be stable pair on files layer
		return &dto.ItemDto{}, OutcomeSame, fmt.Errorf("there is path/guid missmatch")
	}

	if itemByGUID.Guid == "" {
		// Walker.puml "found moved": same hash, the old path no longer exists
		if moved := p.findMovedItem(item, hashShort); moved != nil {
			return moved, OutcomeMoved, p.moveItem(moved, item)
		}

		// Not found, or a duplicate (same hash, the old path still exists): a new item.
		// Reusing a duplicate's thumbnails is a later optimisation.
		created, err := p.CreateItem(item)
		if err != nil {
			return created, OutcomeNew, err
		}
		created.HashShort = hashShort
		_, err = p.UpdateItem(created)
		return created, OutcomeNew, err
	}

	// The type may be known better now (mime step): keep the item's in sync
	if item.MimeType != "" && itemByGUID.MimeType != item.MimeType {
		itemByGUID.MimeType = item.MimeType
		if _, err := p.UpdateItem(itemByGUID); err != nil {
			return itemByGUID, OutcomeSame, err
		}
	}

	if itemByPath.Guid == itemByHash.Guid {
		// all three items are the same, known file
		return itemByGUID, OutcomeSame, nil
	}

	// Same path, new hash (a duplicate of another file with that hash or not):
	// modified, regenerate thumbs
	itemByGUID.State = dto.Dirty
	itemByGUID.HashShort = hashShort
	_, err := p.UpdateItem(itemByGUID)
	return itemByGUID, OutcomeChanged, err
}

func (p *Proxy) getItemsForValidation(file *dto.FileDto, hashShort string) (byGuid *dto.ItemDto, byPath *dto.ItemDto, byHash *dto.ItemDto) {
	var itemByGUID, itemByPath *dto.ItemDto
	var itemsByHash []*dto.ItemDto
	var err error

	if itemByGUID, err = p.GetItemByGuid(file.GUID); err != nil {
		itemByGUID = &dto.ItemDto{}
	}

	if itemByPath, err = p.GetItemByPath(file.Path); err != nil {
		itemByPath = &dto.ItemDto{}
	}

	if itemsByHash, err = p.GetItemsByHash(hashShort); err != nil || len(itemsByHash) == 0 {
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
func (p *Proxy) findMovedItem(file *dto.FileDto, hashShort string) *dto.ItemDto {
	var candidates []*dto.ItemDto
	err := p.db.Unscoped().Where("hash_short = ?", hashShort).
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
func (p *Proxy) moveItem(item *dto.ItemDto, file *dto.FileDto) error {
	oldGuid, newGuid := item.Guid, file.GUID

	err := p.db.Transaction(func(tx *gorm.DB) error {
		// The old main row first: GUID is unique
		if err := tx.Where("guid = ?", oldGuid).Delete(&dto.FileDto{}).Error; err != nil {
			return err
		}
		// Old sidecars that are gone as well; the ones still on disk stay linked
		var oldSidecars []dto.FileDto
		if err := tx.Where("linked_to = ?", oldGuid).Find(&oldSidecars).Error; err != nil {
			return err
		}
		for _, f := range oldSidecars {
			if _, err := os.Stat(f.Path); errors.Is(err, fs.ErrNotExist) {
				if err := tx.Delete(&f).Error; err != nil {
					return err
				}
			}
		}
		// The new group (main + sidecars) links to the item's GUID
		if err := tx.Model(&dto.FileDto{}).Where("linked_to = ?", newGuid).Update("linked_to", oldGuid).Error; err != nil {
			return err
		}
		if err := tx.Model(&dto.FileDto{}).Where("guid = ?", newGuid).Update("guid", oldGuid).Error; err != nil {
			return err
		}

		item.Path = file.Path
		item.MimeType = file.MimeType
		if item.DeletedAt.Valid { // deleted meanwhile: it is back
			item.DeletedAt = gorm.DeletedAt{}
			item.State = dto.Dirty
		}
		return tx.Unscoped().Save(item).Error
	})
	if err != nil {
		return fmt.Errorf("move %s → %s: %w", oldGuid, file.Path, err)
	}

	file.GUID, file.LinkedTo = oldGuid, oldGuid
	return nil
}

// validateKeyed: the key is the item's identity (it never changes, whatever the
// main file is): same hash -> as is; another hash -> Dirty (the main file changed,
// e.g. a derivative replaced by the downloaded original); deleted -> restored.
func (p *Proxy) validateKeyed(key string, main *dto.FileDto, hash string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	err := p.db.Unscoped().Where("guid = ?", key).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = dto.ItemDto{Guid: key, State: dto.New, Path: main.Path, MimeType: main.MimeType, HashShort: hash}
		return &item, p.db.Create(&item).Error
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
	return &item, p.db.Unscoped().Save(&item).Error
}
