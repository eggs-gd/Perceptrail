package model

import (
	"perceptrail/api"
	"perceptrail/gontroller/pkg/model/dto"
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
	ValidateFile(item *dto.FileDto, meta api.RawExif) (*dto.ItemDto, error)

	GetAllItems() ([]*dto.ItemDto, error)
	GetItemByGuid(guid string) (*dto.ItemDto, error)
	GetItemByPath(path string) (*dto.ItemDto, error)
	GetItemByHash(hash string) (*dto.ItemDto, error)
	GetItemsByHash(path string) ([]*dto.ItemDto, error)

	CreateItem(file *dto.FileDto) (*dto.ItemDto, error)

	UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error)
	UpdateItems(items []*dto.ItemDto) ([]*dto.ItemDto, error)
}

func (p *proxy) GetAllItems() ([]*dto.ItemDto, error) {
	var items []*dto.ItemDto
	return items, p.db.Find(&items).Error
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
