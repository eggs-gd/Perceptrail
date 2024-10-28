package model

func (p *Proxy) GetItemByGuid(guid string) (ItemDto, error) {
	var item ItemDto
	return item, p.db.Where("GUID = ?", guid).First(&item).Error
}

func (p *Proxy) GetItemByPath(path string) (ItemDto, error) {
	var item ItemDto
	return item, p.db.Where("path = ?", path).First(&item).Error
}

func (p *Proxy) GetItemByHash(hash string) (ItemDto, error) {
	var item ItemDto
	return item, p.db.Where("hash_short = ?", hash).First(&item).Error
}

func (p *Proxy) GetItemsByHash(hash string) ([]ItemDto, error) {
	var items []ItemDto
	return items, p.db.Where("hash_short = ?", hash).Find(&items).Error
}

func (p *Proxy) CreateItem(file FileDto) (ItemDto, error) {
	item := ItemDto{
		State:    New,
		Path:     file.Path,
		Guid:     file.GUID,
		MimeType: file.MimeType,
	}

	return p.UpdateItem(item)
}

func (p *Proxy) UpdateItem(item ItemDto) (ItemDto, error) {
	return item, p.db.Save(&item).Error
}

func (p *Proxy) UpdateItems(items []ItemDto) ([]ItemDto, error) {
	return items, p.db.Save(items).Error
}
