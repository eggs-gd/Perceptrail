package model

import (
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/google/uuid"
)

type FilesApi interface {
	GetFileByPath(path string) (*dto.FileDto, error)

	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)

	UpdateFile(file *dto.FileDto) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
}

func (p *proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	var file *dto.FileDto = &dto.FileDto{
		GUID:      uuid.New().String(),
		ItemEntry: entry,
	}

	return p.UpdateFile(file)
}

func (p *proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return file, p.db.Save(&file).Error
}

func (p *proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return files, p.db.Save(&files).Error
}

func (p *proxy) GetFileByPath(path string) (*dto.FileDto, error) {
	var file dto.FileDto

	err := p.db.Where("path = ?", path).First(&file).Error
	return &file, err
}
