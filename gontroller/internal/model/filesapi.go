package model

import (
	pubsub "github.com/eggs-gd/go-pub-sub"
	"perceptrail/gontroller/internal/model/dto"

	"github.com/google/uuid"
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

func (t *tx) createFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	var file *dto.FileDto = &dto.FileDto{
		GUID:      uuid.New().String(),
		ItemEntry: entry,
	}

	return t.updateFile(file)
}

func (t *tx) updateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return file, t.db.Save(&file).Error
}

func (t *tx) updateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return files, t.db.Save(&files).Error
}

func (q query) GetFileByPath(path string) (*dto.FileDto, error) {
	var file dto.FileDto

	err := q.db.Where("path = ?", path).First(&file).Error
	return &file, err
}

// FindFile: the file's row, nil when there is none (not an error: a new file)
func (q query) FindFile(path string) (*dto.FileDto, error) {
	var files []*dto.FileDto
	if err := q.db.Where("path = ?", path).Limit(1).Find(&files).Error; err != nil || len(files) == 0 {
		return nil, err
	}
	return files[0], nil
}

// GetAllFiles: every row of the files table (the walk's one read per pass)
func (q query) GetAllFiles() ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	return files, q.db.Find(&files).Error
}

// GetFilesByID: the rows of these IDs as they are now (an ID whose row is gone
// gives nothing)
func (q query) GetFilesByID(ids []uint) ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	for start := 0; start < len(ids); start += 500 { // under SQLite's variable limit
		var page []*dto.FileDto
		if err := q.db.Where("id IN ?", ids[start:min(start+500, len(ids))]).Find(&page).Error; err != nil {
			return nil, err
		}
		files = append(files, page...)
	}
	return files, nil
}

// createFiles: the rows of new files, in one transaction; each gets its GUID and
// is marked Changed (new work)
func (t *tx) createFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	files := make([]*dto.FileDto, len(entries))
	for i, e := range entries {
		files[i] = &dto.FileDto{GUID: uuid.New().String(), ItemEntry: e, Changed: true}
	}
	return files, t.db.CreateInBatches(files, 200).Error
}

// saveStats: the new stat of changed files (size, mtime) and their Changed mark —
// only those columns (the rest of a row may have moved on since the walk read
// it)
func (t *tx) saveStats(files []*dto.FileDto) (pubsub.None, error) {
	if len(files) == 0 {
		return pubsub.None{}, nil
	}
	for _, f := range files {
		err := t.db.Model(&dto.FileDto{}).Where("id = ?", f.ID).
			UpdateColumns(map[string]any{"size": f.Size, "mod_time": f.ModTime, "changed": f.Changed}).Error
		if err != nil {
			return pubsub.None{}, err
		}
	}
	return pubsub.None{}, nil
}

func (t *tx) deleteFiles(files []*dto.FileDto) (pubsub.None, error) {
	if len(files) == 0 {
		return pubsub.None{}, nil
	}
	return pubsub.None{}, t.db.Delete(&files).Error
}

func (t *tx) unignoreFiles(pubsub.None) (int64, error) {
	res := t.db.Model(&dto.FileDto{}).Where("linked_to = ?", "-").Update("linked_to", "")
	return res.RowsAffected, res.Error
}

func (q query) GetLinkedFiles(guid string) ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	return files, q.db.Where("linked_to = ?", guid).Order("id").Find(&files).Error
}

func (q query) CountLinkedFiles(guid string) (int64, error) {
	var n int64
	err := q.db.Model(&dto.FileDto{}).Where("linked_to = ?", guid).Count(&n).Error
	return n, err
}

func (q query) GetFileByID(id uint) (*dto.FileDto, error) {
	var f dto.FileDto
	return &f, q.db.First(&f, id).Error
}
