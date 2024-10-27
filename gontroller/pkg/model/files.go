package model

import (
	t "gontroller/pkg/_t"

	"github.com/google/uuid"
)

func (p *Proxy) CreateFile(entry t.ItemEntry) (FileDto, error) {
	var file FileDto = FileDto{
		GUID:      uuid.New().String(),
		ItemEntry: entry,
	}

	return p.UpdateFile(file)
}

func (p *Proxy) UpdateFile(file FileDto) (FileDto, error) {
	return file, p.db.Save(file).Error
}

func (p *Proxy) UpdateFiles(files []FileDto) ([]FileDto, error) {
	return files, p.db.Save(files).Error
}

func (p *Proxy) GetFileByPath(path string) (FileDto, error) {
	var file FileDto

	err := p.db.Where("path = ?", path).First(&file).Error
	return file, err
}
