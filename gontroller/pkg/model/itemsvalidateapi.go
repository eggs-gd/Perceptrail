package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"perceptrail/gontroller/pkg/model/dto"
	"sort"

	"github.com/eggs-gd/perceplib/api"
	"gorm.io/gorm"
)

// File-system tags (exiftool File group) that change on move/rename/read
// and must not affect content identity.
var volatileHashTags = map[string]struct{}{
	"FileName":            {},
	"Directory":           {},
	"FileModifyDate":      {},
	"FileAccessDate":      {},
	"FileInodeChangeDate": {},
	"FileCreateDate":      {},
	"FilePermissions":     {},
	"FileAttributes":      {},
}

func (p *proxy) getShortHash(item *dto.FileDto, meta api.RawExif) string {
	h := sha256.New()

	// Write the file size to the hash
	h.Write([]byte(fmt.Sprintf("%d", item.Size)))

	// Process the EXIF data
	var keys []string
	for k := range meta {
		if _, skip := volatileHashTags[k]; skip {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(fmt.Sprintf("%d", meta[k])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (p *proxy) ValidateFile(item *dto.FileDto, meta api.RawExif) (*dto.ItemDto, error) {
	var hashShort = p.getShortHash(item, meta)
	var err error

	itemByGUID, itemByPath, itemByHash := p.getItemsForValidation(item, hashShort)

	if itemByGUID.Guid != itemByPath.Guid {
		// probably panic(). Path/Guid should be stable pair on files layer
		return &dto.ItemDto{}, fmt.Errorf("there is path/guid missmatch")
	}

	if itemByGUID.Guid == "" {
		// Walker.puml "found moved": same hash, the old path no longer exists
		if moved := p.findMovedItem(item, hashShort); moved != nil {
			return moved, p.moveItem(moved, item)
		}

		// Not found, or a duplicate (same hash, the old path still exists): a new item.
		// if this guid/path is not present in DB we don't care about duplicates and just create new one for now
		// for future I kept commented out code for optimisation with available duplicates and their thumbs
		// make sense only in CPU environments with a lot of transcoding work
		itemByGUID, err = p.CreateItem(item)
		itemByGUID.HashShort = hashShort
		p.UpdateItem(itemByGUID)
		return itemByGUID, err

		/*
			if itemByPath.GUID == "" && itemByHash.GUID == "" {
				// all three items are empty, new file
				// create item
				return NewFile, nil
			}

			if itemByHash.GUID != "" {
				// path not exiss, hash exists
				// file moved. It is in list of Items but with different path
				// new duplicate file
				// todo for future -> provide some optimisation for thumbs/transcodes reusing between duplicates
				return MovedFile, nil
			}
		*/

	} else {
		if itemByPath.Guid == itemByHash.Guid {
			// all three items are the same, Known file
			// do nothing
			return itemByGUID, nil
		}

		if itemByHash.Guid == "" {
			// hash for known file changed and not exists
			// modified, update short hash itemByGuid.HashShort = hashShort, regenerate thumbs
			itemByGUID.State = dto.Dirty
			itemByGUID.HashShort = hashShort
			p.UpdateItem(itemByGUID)
			return itemByGUID, nil
		}

		if itemByHash.Guid != itemByPath.Guid {
			// path != hash but both exists
			// ItemByGuid was modified but we have the same hash on another file
			// threat as modified, ignore duplicate for now
			// todo for future -> provide some optimisation for thumbs/transcodes reusing between duplicates
			itemByGUID.State = dto.Dirty
			itemByGUID.HashShort = hashShort
			p.UpdateItem(itemByGUID)
			return itemByGUID, nil
		}
	}

	return &dto.ItemDto{}, fmt.Errorf("something unknown went wrong in validator")
}

func (p *proxy) getItemsForValidation(file *dto.FileDto, hashShort string) (byGuid *dto.ItemDto, byPath *dto.ItemDto, byHash *dto.ItemDto) {
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
func (p *proxy) findMovedItem(file *dto.FileDto, hashShort string) *dto.ItemDto {
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
func (p *proxy) moveItem(item *dto.ItemDto, file *dto.FileDto) error {
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
