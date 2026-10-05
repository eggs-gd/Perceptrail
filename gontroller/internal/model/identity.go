package model

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	l "github.com/eggs-gd/go-zap-decor"
	"gorm.io/gorm"
)

// Identity: which item a group of files is — by the main file's path and
// fingerprint in a plain folder (the same, moved, new or changed: Walker.puml), by
// its key where the source knows it (an Apple Photos asset). The item side; the
// files' side (linking, relinking) is in files.go.

// ValidateGroupArgs: a plain folder's group (the main file first) and its
// fingerprint
type ValidateGroupArgs struct {
	Files []*dto.FileDto
	Hash  string
}

// ValidateAssetArgs: a keyed group — its key, its files, its fingerprint
type ValidateAssetArgs struct {
	Key   string
	Files []*dto.FileDto
	Hash  string
}

// identityCommands: identity's writes as commands
type identityCommands struct {
	validateGroup op[ValidateGroupArgs, *dto.ItemDto]
	validateAsset op[ValidateAssetArgs, *dto.ItemDto]
}

func (p *Proxy) ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return p.validateGroup.Do(ValidateGroupArgs{files, hash})
}

func (p *Proxy) ValidateGroupCommand() pubsub.Command[ValidateGroupArgs, *dto.ItemDto] {
	return p.validateGroup
}

func (p *Proxy) ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return p.validateAsset.Do(ValidateAssetArgs{key, files, hash})
}

func (p *Proxy) ValidateAssetCommand() pubsub.Command[ValidateAssetArgs, *dto.ItemDto] {
	return p.validateAsset
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
// and client links are keyed by it), its files are the new group's now
// (relinkGroup). A deleted item is restored.
func (t *tx) moveItem(item *dto.ItemDto, file *dto.FileDto) error {
	if err := t.relinkGroup(file.GUID, item.Guid); err != nil {
		return fmt.Errorf("move %s → %s: %w", item.Guid, file.Path, err)
	}
	item.Path = file.Path
	item.MimeType = file.MimeType
	if item.DeletedAt.Valid { // deleted meanwhile: it is back
		item.DeletedAt = gorm.DeletedAt{}
		item.State = dto.Dirty
	}
	if err := t.db.Unscoped().Save(item).Error; err != nil {
		return fmt.Errorf("move %s → %s: %w", item.Guid, file.Path, err)
	}
	file.GUID, file.LinkedTo = item.Guid, item.Guid
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

func newIdentityCommands(p *Proxy) identityCommands {
	return identityCommands{
		validateGroup: command(p, pubsub.Frame, (*tx).validateGroup),
		validateAsset: command(p, pubsub.Frame, (*tx).validateAsset),
	}
}
