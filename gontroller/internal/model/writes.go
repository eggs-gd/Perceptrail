package model

import (
	"perceptrail/gontroller/internal/model/dto"
)

// The model's writes: each runs its rule — the unexported method of the same name
// — in the writer's transaction (write). Inside a rule, these run right there, in
// the same transaction.

func (p *Proxy) ClearHashes() (int64, error) {
	return written(p, func(q *Proxy) (int64, error) { return q.clearHashes() })
}

func (p *Proxy) CreateItem(file *dto.FileDto) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) { return q.createItem(file) })
}

func (p *Proxy) UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) { return q.updateItem(item) })
}

func (p *Proxy) DeleteItem(item *dto.ItemDto) error {
	return p.write(func(q *Proxy) error { return q.deleteItem(item) })
}

func (p *Proxy) Gone(files []*dto.FileDto) (deleted, dirty int, err error) {
	err = p.write(func(q *Proxy) (err error) {
		deleted, dirty, err = q.gone(files)
		return err
	})
	return deleted, dirty, err
}

func (p *Proxy) Ignore(files []*dto.FileDto) error {
	return p.write(func(q *Proxy) error { return q.ignore(files) })
}

func (p *Proxy) ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) { return q.validateGroup(files, hash) })
}

func (p *Proxy) ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) { return q.validateAsset(key, files, hash) })
}

func (p *Proxy) Publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	return written(p, func(q *Proxy) (*dto.ItemDto, error) { return q.publish(item) })
}

func (p *Proxy) MarkRework(guids []string) (int64, error) {
	return written(p, func(q *Proxy) (int64, error) { return q.markRework(guids) })
}

func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	return written(p, func(q *Proxy) (*dto.FileDto, error) { return q.createFile(entry) })
}

func (p *Proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return written(p, func(q *Proxy) (*dto.FileDto, error) { return q.updateFile(file) })
}

func (p *Proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return written(p, func(q *Proxy) ([]*dto.FileDto, error) { return q.updateFiles(files) })
}

func (p *Proxy) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	return written(p, func(q *Proxy) ([]*dto.FileDto, error) { return q.createFiles(entries) })
}

func (p *Proxy) SaveStats(files []*dto.FileDto) error {
	return p.write(func(q *Proxy) error { return q.saveStats(files) })
}

func (p *Proxy) DeleteFiles(files []*dto.FileDto) error {
	return p.write(func(q *Proxy) error { return q.deleteFiles(files) })
}

func (p *Proxy) UnignoreFiles() (int64, error) {
	return written(p, func(q *Proxy) (int64, error) { return q.unignoreFiles() })
}

func (p *Proxy) SetMeta(key, value string) error {
	return p.write(func(q *Proxy) error { return q.setMeta(key, value) })
}
