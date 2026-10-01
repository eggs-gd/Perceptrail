package model

import (
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

type ItemsApi interface {
	// GetShortHash the idea is to hash only size in bytes and whole set of metadata fields
	// Means that expecting if something changed - size in bytes will be different
	// If exifdata changed - full hash will be different
	// Not perfect but as another one gate in bunch of sequential checks:
	// - file scanner gate,
	// - short hash gate,
	// - full hash gate
	//GetShortHash(rawExif t.RawExif, fileSizeBytes uint64) string
	ValidateFile(item *dto.FileDto, meta api.RawExif) (*dto.ItemDto, Outcome, error)
	// ValidateKeyed: the item of a group whose source knows its identity (an Apple
	// Photos asset UUID); found by the key — restored if deleted — or created
	ValidateKeyed(key string, main *dto.FileDto, meta api.RawExif) (*dto.ItemDto, error)

	GetAllItems() ([]*dto.ItemDto, error)
	// StreamAllItems walks items newest first (the default sheet: date) without
	// loading the full table into memory; every item comes with its files (one query
	// per page)
	StreamAllItems(fn func(*dto.ItemDto, []*dto.FileDto) error) error
	// GetItemsInStates: the items in these states, with what navigators read (guid,
	// date and its zone, size) — not the whole rows
	GetItemsInStates(states ...dto.ItemState) ([]*dto.ItemDto, error)
	GetItemByGuid(guid string) (*dto.ItemDto, error)
	GetItemByPath(path string) (*dto.ItemDto, error)
	GetItemByHash(hash string) (*dto.ItemDto, error)
	GetItemsByHash(path string) ([]*dto.ItemDto, error)

	CreateItem(file *dto.FileDto) (*dto.ItemDto, error)

	UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error)
	UpdateItems(items []*dto.ItemDto) ([]*dto.ItemDto, error)

	// DeleteItem marks the item Deleted and soft-deletes it (hidden from queries)
	DeleteItem(item *dto.ItemDto) error
}

func (p *proxy) GetAllItems() ([]*dto.ItemDto, error) {
	var items []*dto.ItemDto
	return items, p.db.Find(&items).Error
}

func (p *proxy) StreamAllItems(fn func(*dto.ItemDto, []*dto.FileDto) error) error {
	const pageSize = 32

	// The ids in sheet order first (cheap), then the items page by page: a stable
	// order while the import writes, without paging by a date
	var ids []uint
	if err := p.db.Model(&dto.ItemDto{}).Order("date DESC, id DESC").Pluck("id", &ids).Error; err != nil {
		return err
	}

	for start := 0; start < len(ids); start += pageSize {
		page := ids[start:min(start+pageSize, len(ids))]
		var batch []*dto.ItemDto
		if err := p.db.Where("id IN ?", page).Find(&batch).Error; err != nil {
			return err
		}
		byID := make(map[uint]*dto.ItemDto, len(batch))
		guids := make([]string, len(batch))
		for i, it := range batch {
			byID[it.ID] = it
			guids[i] = it.Guid
		}
		var files []*dto.FileDto
		if err := p.db.Where("linked_to IN ?", guids).Order("id").Find(&files).Error; err != nil {
			return err
		}
		byItem := map[string][]*dto.FileDto{}
		for _, f := range files {
			byItem[f.LinkedTo] = append(byItem[f.LinkedTo], f)
		}

		for _, id := range page {
			it, ok := byID[id]
			if !ok {
				continue // deleted meanwhile
			}
			if err := fn(it, byItem[it.Guid]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *proxy) GetItemsInStates(states ...dto.ItemState) ([]*dto.ItemDto, error) {
	var items []*dto.ItemDto
	return items, p.db.Select("id", "guid", "date", "date_offset", "date_source", "size_w", "size_h", "ratio_w", "ratio_h").
		Where("state IN ?", states).Find(&items).Error
}

func (p *proxy) GetItemByGuid(guid string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	return &item, p.db.Where("guid = ?", guid).First(&item).Error
}

func (p *proxy) GetItemByPath(path string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	return &item, p.db.Where("path = ?", path).First(&item).Error
}

func (p *proxy) GetItemByHash(hash string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	return &item, p.db.Where("hash_short = ?", hash).First(&item).Error
}

func (p *proxy) GetItemsByHash(hash string) ([]*dto.ItemDto, error) {
	var items []*dto.ItemDto
	return items, p.db.Where("hash_short = ?", hash).Find(&items).Error
}

func (p *proxy) CreateItem(file *dto.FileDto) (*dto.ItemDto, error) {
	item := dto.ItemDto{
		State:    dto.New,
		Path:     file.Path,
		Guid:     file.GUID,
		MimeType: file.MimeType,
	}

	return p.UpdateItem(&item)
}

func (p *proxy) UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return item, p.db.Save(&item).Error
}

func (p *proxy) UpdateItems(items []*dto.ItemDto) ([]*dto.ItemDto, error) {
	return items, p.db.Save(&items).Error
}

func (p *proxy) DeleteItem(item *dto.ItemDto) error {
	item.State = dto.Deleted
	if err := p.db.Save(item).Error; err != nil {
		return err
	}
	return p.db.Delete(item).Error
}
