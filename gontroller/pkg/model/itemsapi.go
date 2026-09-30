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
	// StreamAllItems walks items via a DB cursor without loading the full table into
	// memory; every item comes with its files (one query per page)
	StreamAllItems(fn func(*dto.ItemDto, []*dto.FileDto) error) error
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
	var lastID uint

	for {
		var batch []dto.ItemDto
		q := p.db.Model(&dto.ItemDto{}).Order("id").Limit(pageSize)
		if lastID > 0 {
			q = q.Where("id > ?", lastID)
		}
		if err := q.Find(&batch).Error; err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}

		guids := make([]string, len(batch))
		for i := range batch {
			guids[i] = batch[i].Guid
		}
		var files []*dto.FileDto
		if err := p.db.Where("linked_to IN ?", guids).Order("id").Find(&files).Error; err != nil {
			return err
		}
		byItem := map[string][]*dto.FileDto{}
		for _, f := range files {
			byItem[f.LinkedTo] = append(byItem[f.LinkedTo], f)
		}

		for i := range batch {
			if err := fn(&batch[i], byItem[batch[i].Guid]); err != nil {
				return err
			}
			lastID = batch[i].ID
		}
	}
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
