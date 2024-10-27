package model

import "github.com/google/uuid"

func (p *Proxy) GetItemPathHash(path string, hashShort string) (ItemDto, error) {
	var item ItemDto

	err := p.db.Where("path = ?", path).Or("hash_short = ?", hashShort).First(&item).Error
	return item, err
}

func (p *Proxy) UpdateItem(item ItemDto) error {
	return p.db.Save(&item).Error
}

func (p *Proxy) CreateItem(item ItemDto) error {
	item.GUID = uuid.New().String()

	return p.db.Create(&item).Error
}
