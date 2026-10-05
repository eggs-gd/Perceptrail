package model

import (
	"errors"
	"io/fs"
	"os"
	"uuid"

	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
)

// Files: the rows of the files on disk, each linked to its item (LinkedTo) or
// ignored. Reads, the writes on tx, their commands and their two public faces.

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

func (q query) GetFileByID(id uint) (*dto.FileDto, error) {
	var f dto.FileDto
	return &f, q.db.First(&f, id).Error
}

// GetLinkedFiles: every file of an item (its group: sidecars, derivatives…)
func (q query) GetLinkedFiles(guid string) ([]*dto.FileDto, error) {
	var files []*dto.FileDto
	return files, q.db.Where("linked_to = ?", guid).Order("id").Find(&files).Error
}

// CountLinkedFiles: how many files an item still has
func (q query) CountLinkedFiles(guid string) (int64, error) {
	var n int64
	err := q.db.Model(&dto.FileDto{}).Where("linked_to = ?", guid).Count(&n).Error
	return n, err
}

func (t *tx) createFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	var file *dto.FileDto = &dto.FileDto{
		GUID:      uuid.New().String(),
		ItemEntry: entry,
	}

	return t.updateFile(file)
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

func (t *tx) updateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return file, t.db.Save(&file).Error
}

func (t *tx) updateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return files, t.db.Save(&files).Error
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

// relinkGroup: a moved item's files — the group of the new main file (its GUID:
// group) takes over the item's GUID; the item's old main row goes (GUID is unique),
// and its old sidecars that are gone as well (the ones still on disk stay linked)
func (t *tx) relinkGroup(group, item string) error {
	if err := t.db.Where("guid = ?", item).Delete(&dto.FileDto{}).Error; err != nil {
		return err
	}
	var oldSidecars []dto.FileDto
	if err := t.db.Where("linked_to = ?", item).Find(&oldSidecars).Error; err != nil {
		return err
	}
	for _, f := range oldSidecars {
		if _, err := os.Stat(f.Path); errors.Is(err, fs.ErrNotExist) {
			if err := t.db.Delete(&f).Error; err != nil {
				return err
			}
		}
	}
	// The new group (main + sidecars) links to the item's GUID
	if err := t.db.Model(&dto.FileDto{}).Where("linked_to = ?", group).Update("linked_to", item).Error; err != nil {
		return err
	}
	return t.db.Model(&dto.FileDto{}).Where("guid = ?", group).Update("guid", item).Error
}

// fileCommands: the files' writes as commands
type fileCommands struct {
	createFile    op[dto.ItemEntry, *dto.FileDto]
	createFiles   op[[]dto.ItemEntry, []*dto.FileDto]
	updateFile    op[*dto.FileDto, *dto.FileDto]
	updateFiles   op[[]*dto.FileDto, []*dto.FileDto]
	saveStats     op[[]*dto.FileDto, pubsub.None]
	deleteFiles   op[[]*dto.FileDto, pubsub.None]
	unignoreFiles op[pubsub.None, int64]
}

func newFileCommands(p *Proxy) fileCommands {
	return fileCommands{
		createFile:    command(p, pubsub.Frame, (*tx).createFile),
		createFiles:   command(p, pubsub.Frame, (*tx).createFiles),
		updateFile:    command(p, pubsub.Frame, (*tx).updateFile),
		updateFiles:   command(p, pubsub.Frame, (*tx).updateFiles),
		saveStats:     command(p, pubsub.Frame, (*tx).saveStats),
		deleteFiles:   command(p, pubsub.Frame, (*tx).deleteFiles),
		unignoreFiles: command(p, pubsub.Frame, (*tx).unignoreFiles),
	}
}

func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	return p.createFile.Do(entry)
}

func (p *Proxy) CreateFileCommand() pubsub.Command[dto.ItemEntry, *dto.FileDto] {
	return p.createFile
}

func (p *Proxy) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	return p.createFiles.Do(entries)
}

func (p *Proxy) CreateFilesCommand() pubsub.Command[[]dto.ItemEntry, []*dto.FileDto] {
	return p.createFiles
}

func (p *Proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return p.updateFile.Do(file)
}

func (p *Proxy) UpdateFileCommand() pubsub.Command[*dto.FileDto, *dto.FileDto] {
	return p.updateFile
}

func (p *Proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return p.updateFiles.Do(files)
}

func (p *Proxy) UpdateFilesCommand() pubsub.Command[[]*dto.FileDto, []*dto.FileDto] {
	return p.updateFiles
}

func (p *Proxy) SaveStats(files []*dto.FileDto) error {
	_, err := p.saveStats.Do(files)
	return err
}

func (p *Proxy) SaveStatsCommand() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.saveStats)
}

func (p *Proxy) DeleteFiles(files []*dto.FileDto) error {
	_, err := p.deleteFiles.Do(files)
	return err
}

func (p *Proxy) DeleteFilesCommand() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.deleteFiles)
}

// UnignoreFiles clears the "ignored" mark: those groups are classified again
func (p *Proxy) UnignoreFiles() (int64, error) {
	return p.unignoreFiles.Do(pubsub.None{})
}

func (p *Proxy) UnignoreFilesCommand() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.unignoreFiles)
}
