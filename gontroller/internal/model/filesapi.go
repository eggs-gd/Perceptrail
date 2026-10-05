package model

import (
	"perceptrail/gontroller/internal/model/dto"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FilesApi interface {
	GetFileByPath(path string) (*dto.FileDto, error)

	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)

	UpdateFile(file *dto.FileDto) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)

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

// FindFile: the file's row, nil when there is none (not an error: a new file)
func (p *Proxy) FindFile(path string) (*dto.FileDto, error) {
	var files []*dto.FileDto
	if err := p.db.Where("path = ?", path).Limit(1).Find(&files).Error; err != nil || len(files) == 0 {
		return nil, err
	}
	return files[0], nil
}

// GetAllFiles: every row of the files table (the walk's one read per pass)
func (p *Proxy) GetAllFiles() ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	return files, p.db.Find(&files).Error
}

// GetFilesByID: the rows of these IDs as they are now (an ID whose row is gone
// gives nothing)
func (p *Proxy) GetFilesByID(ids []uint) ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	for start := 0; start < len(ids); start += 500 { // under SQLite's variable limit
		var page []*dto.FileDto
		if err := p.db.Where("id IN ?", ids[start:min(start+500, len(ids))]).Find(&page).Error; err != nil {
			return nil, err
		}
		files = append(files, page...)
	}
	return files, nil
}

// CreateFiles: the rows of new files, in one transaction; each gets its GUID and
// is marked Changed (new work)
func (p *Proxy) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	files := make([]*dto.FileDto, len(entries))
	for i, e := range entries {
		files[i] = &dto.FileDto{GUID: uuid.New().String(), ItemEntry: e, Changed: true}
	}
	return files, p.db.CreateInBatches(files, 200).Error
}

// SaveStats: the new stat of changed files (size, mtime) and their Changed mark —
// only those columns (the rest of a row may have moved on since the walk read
// it), in one transaction
func (p *Proxy) SaveStats(files []*dto.FileDto) error {
	if len(files) == 0 {
		return nil
	}
	return p.db.Transaction(func(tx *gorm.DB) error {
		for _, f := range files {
			err := tx.Model(&dto.FileDto{}).Where("id = ?", f.ID).
				UpdateColumns(map[string]any{"size": f.Size, "mod_time": f.ModTime, "changed": f.Changed}).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
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
