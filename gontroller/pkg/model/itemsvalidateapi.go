package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	t "perceptrail/gontroller/pkg/_t"
	"perceptrail/gontroller/pkg/model/dto"
	"sort"
)

func (p *Proxy) getShortHash(item *dto.FileDto, meta t.RawExif) string {
	h := sha256.New()

	// Write the file size to the hash
	h.Write([]byte(fmt.Sprintf("%d", &item.Size)))

	// Process the EXIF data
	var keys []string
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(fmt.Sprintf("%d", meta[k])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (p *Proxy) ValidateFile(item *dto.FileDto, meta t.RawExif) (*dto.ItemDto, error) {
	var hashShort = p.getShortHash(item, meta)
	var err error

	itemByGUID, itemByPath, itemByHash := p.getItemsForValidation(item, hashShort)

	if itemByGUID.Guid != itemByPath.Guid {
		// probably panic(). Path/Guid should be stable pair on files layer
		return &dto.ItemDto{}, fmt.Errorf("there is path/guid missmatch")
	}

	if itemByGUID.Guid == "" {
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
