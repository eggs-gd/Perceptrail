package model

import (
	t "gontroller/pkg/_t"
	"gontroller/pkg/model/dto"

	"github.com/google/uuid"
)

type FilesApi interface {
	GetFileByPath(path string) (dto.FileDto, error)

	CreateFile(entry t.ItemEntry) (dto.FileDto, error)

	UpdateFile(file dto.FileDto) (dto.FileDto, error)
	UpdateFiles(files []dto.FileDto) ([]dto.FileDto, error)
}

func (p *Proxy) CreateFile(entry t.ItemEntry) (dto.FileDto, error) {
	var file dto.FileDto = dto.FileDto{
		GUID:      uuid.New().String(),
		ItemEntry: entry,
	}

	return p.UpdateFile(file)
}

func (p *Proxy) UpdateFile(file dto.FileDto) (dto.FileDto, error) {
	return file, p.db.Save(&file).Error
}

func (p *Proxy) UpdateFiles(files []dto.FileDto) ([]dto.FileDto, error) {
	return files, p.db.Save(&files).Error
}

func (p *Proxy) GetFileByPath(path string) (dto.FileDto, error) {
	var file dto.FileDto

	err := p.db.Where("path = ?", path).First(&file).Error
	return file, err
}
