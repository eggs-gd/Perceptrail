package model

import (
	"perceptrail/gontroller/internal/model/dto"
	"time"

	"github.com/google/uuid"
)

type FilesApi interface {
	GetFileByPath(path string) (*dto.FileDto, error)
	// FindFile: the file's row, nil when there is none (not an error: a new file)
	FindFile(path string) (*dto.FileDto, error)

	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)

	UpdateFile(file *dto.FileDto) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)

	// GetFilesCheckedBefore returns files not seen by the walk that started at t
	GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error)
	DeleteFiles(files []*dto.FileDto) error
	// UnignoreFiles clears the "ignored" mark: those groups are classified again
	UnignoreFiles() (int64, error)
	// CountLinkedFiles: how many files an item still has
	CountLinkedFiles(guid string) (int64, error)
	// GetLinkedFiles: every file of an item (its group: sidecars, derivatives…)
	GetLinkedFiles(guid string) ([]*dto.FileDto, error)
	GetFileByID(id uint) (*dto.FileDto, error)
}

func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	var file *dto.FileDto = &dto.FileDto{
		GUID:      uuid.New().String(),
		ItemEntry: entry,
	}

	return p.UpdateFile(file)
}

func (p *Proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return file, p.db.Save(&file).Error
}

func (p *Proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return files, p.db.Save(&files).Error
}

func (p *Proxy) GetFileByPath(path string) (*dto.FileDto, error) {
	var file dto.FileDto

	err := p.db.Where("path = ?", path).First(&file).Error
	return &file, err
}

func (p *Proxy) FindFile(path string) (*dto.FileDto, error) {
	var files []*dto.FileDto
	if err := p.db.Where("path = ?", path).Limit(1).Find(&files).Error; err != nil || len(files) == 0 {
		return nil, err
	}
	return files[0], nil
}

func (p *Proxy) GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	if err := p.db.Find(&files).Error; err != nil {
		return nil, err
	}
	// Compared in Go: the driver stores times as text with the local offset, which
	// differs across DST changes, so a SQL string comparison is not reliable
	gone := files[:0]
	for _, f := range files {
		if f.CheckTime.Before(t) {
			gone = append(gone, f)
		}
	}
	return gone, nil
}

func (p *Proxy) DeleteFiles(files []*dto.FileDto) error {
	if len(files) == 0 {
		return nil
	}
	return p.db.Delete(&files).Error
}

func (p *Proxy) UnignoreFiles() (int64, error) {
	res := p.db.Model(&dto.FileDto{}).Where("linked_to = ?", "-").Update("linked_to", "")
	return res.RowsAffected, res.Error
}

func (p *Proxy) GetLinkedFiles(guid string) ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	return files, p.db.Where("linked_to = ?", guid).Order("id").Find(&files).Error
}

func (p *Proxy) CountLinkedFiles(guid string) (int64, error) {
	var n int64
	err := p.db.Model(&dto.FileDto{}).Where("linked_to = ?", guid).Count(&n).Error
	return n, err
}

func (p *Proxy) GetFileByID(id uint) (*dto.FileDto, error) {
	var f dto.FileDto
	return &f, p.db.First(&f, id).Error
}
