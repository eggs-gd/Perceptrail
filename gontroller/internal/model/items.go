package model

import (
	"time"

	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	"gorm.io/gorm"
)

// Items: what the library shows — one per photo or video, whatever its files.
// Reads, the writes on tx, their commands and their two public faces.

// Deletions are sent this much earlier than asked: some deleted_at rows carry no
// zone (written as local time); a tombstone too many is harmless
const deletionMargin = 24 * time.Hour

// itemCommands: the items' writes as commands
type itemCommands struct {
	createItem  op[*dto.FileDto, *dto.ItemDto]
	updateItem  op[*dto.ItemDto, *dto.ItemDto]
	deleteItem  op[*dto.ItemDto, pubsub.None]
	clearHashes op[pubsub.None, int64]
}

// GetAllGuids: the GUIDs of every item (deleted ones excluded)
func (q query) GetAllGuids() ([]string, error) {
	var guids []string
	return guids, q.db.Model(&dto.ItemDto{}).Pluck("guid", &guids).Error
}

// StreamAllItems walks items newest first (the default sheet: date) without
// loading the full table into memory; every item comes with its files (one query
// per page)
func (q query) StreamAllItems(fn func(*dto.ItemDto, []*dto.FileDto) error) error {
	// The ids in sheet order first (cheap), then the items page by page: a stable
	// order while the import writes, without paging by a date
	var ids []uint
	if err := q.db.Model(&dto.ItemDto{}).Order("date DESC, id DESC").Pluck("id", &ids).Error; err != nil {
		return err
	}
	return q.streamIDs(q.db, ids, fn)
}

// StreamItemsSince: the items changed or deleted since then (deleted ones too:
// DeletedAt is set), for the client's delta sync
func (q query) StreamItemsSince(since time.Time, fn func(*dto.ItemDto, []*dto.FileDto) error) error {
	// SQLite keeps times as text with the writer's offset ("…+03:00", across DST
	// changes too): compared as julian days, not as text
	var ids []uint
	err := q.db.Unscoped().Model(&dto.ItemDto{}).
		Where("julianday(updated_at) >= julianday(?) OR julianday(deleted_at) >= julianday(?)",
			since.UTC(), since.Add(-deletionMargin).UTC()).
		Order("id").Pluck("id", &ids).Error
	if err != nil {
		return err
	}
	return q.streamIDs(q.db.Unscoped(), ids, fn)
}

// GetItemsInStates: the items in these states, newest first, with what
// perceptors read to order them (guid, date and its zone, size, duration) and
// where they come from (path, kind) — not the whole rows
func (q query) GetItemsInStates(states ...dto.ItemState) ([]*dto.ItemDto, error) {
	var items []*dto.ItemDto
	return items, q.db.Select("id", "guid", "date", "date_offset", "date_source", "size_w", "size_h", "ratio_w", "ratio_h", "duration", "path", "kind").
		Where("state IN ?", states).Order("date DESC, id DESC").Find(&items).Error
}

// CountItemsInStates: how many items are in these states (deleted ones excluded)
func (q query) CountItemsInStates(states ...dto.ItemState) (int64, error) {
	var n int64
	return n, q.db.Model(&dto.ItemDto{}).Where("state IN ?", states).Count(&n).Error
}

func (q query) GetItemByGuid(guid string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	return &item, q.db.Where("guid = ?", guid).First(&item).Error
}

func (q query) GetItemByPath(path string) (*dto.ItemDto, error) {
	var item dto.ItemDto
	return &item, q.db.Where("path = ?", path).First(&item).Error
}

func (q query) GetItemsByHash(hash string) ([]*dto.ItemDto, error) {
	var items []*dto.ItemDto
	return items, q.db.Where("hash_short = ?", hash).Find(&items).Error
}

func (p *Proxy) CreateItem(file *dto.FileDto) (*dto.ItemDto, error) {
	return p.createItem.Do(file)
}

func (p *Proxy) CreateItemCommand() pubsub.Command[*dto.FileDto, *dto.ItemDto] {
	return p.createItem
}

func (p *Proxy) UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return p.updateItem.Do(item)
}

func (p *Proxy) UpdateItemCommand() pubsub.Command[*dto.ItemDto, *dto.ItemDto] {
	return p.updateItem
}

// DeleteItem marks the item Deleted and soft-deletes it (hidden from queries)
func (p *Proxy) DeleteItem(item *dto.ItemDto) error {
	_, err := p.deleteItem.Do(item)
	return err
}

func (p *Proxy) DeleteItemCommand() pubsub.Message[*dto.ItemDto] {
	return pubsub.MessageOf(p.deleteItem)
}

// ClearHashes: every item forgets its fingerprint (the fingerprint changed): the
// gate sends each group once more to get the new one. Not a change the client
// sees: updated_at stays (a delta would stream the whole library)
func (p *Proxy) ClearHashes() (int64, error) {
	return p.clearHashes.Do(pubsub.None{})
}

func (p *Proxy) ClearHashesCommand() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.clearHashes)
}

// streamIDs: the items of ids in that order, each with its files (one query per page)
func (q query) streamIDs(db *gorm.DB, ids []uint, fn func(*dto.ItemDto, []*dto.FileDto) error) error {
	const pageSize = 32
	for start := 0; start < len(ids); start += pageSize {
		page := ids[start:min(start+pageSize, len(ids))]
		var batch []*dto.ItemDto
		if err := db.Where("id IN ?", page).Find(&batch).Error; err != nil {
			return err
		}
		byID := make(map[uint]*dto.ItemDto, len(batch))
		guids := make([]string, len(batch))
		for i, it := range batch {
			byID[it.ID] = it
			guids[i] = it.Guid
		}
		var files []*dto.FileDto
		if err := q.db.Where("linked_to IN ?", guids).Order("id").Find(&files).Error; err != nil {
			return err
		}
		byItem := map[string][]*dto.FileDto{}
		for _, f := range files {
			byItem[f.LinkedTo] = append(byItem[f.LinkedTo], f)
		}

		for _, id := range page {
			it, ok := byID[id]
			if !ok {
				continue // deleted meanwhile
			}
			if err := fn(it, byItem[it.Guid]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *tx) createItem(file *dto.FileDto) (*dto.ItemDto, error) {
	item := dto.ItemDto{
		State:    dto.New,
		Path:     file.Path,
		Guid:     file.GUID,
		MimeType: file.MimeType,
	}

	return t.updateItem(&item)
}

func (t *tx) updateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return item, t.db.Save(&item).Error
}

func (t *tx) deleteItem(item *dto.ItemDto) (pubsub.None, error) {
	item.State = dto.Deleted
	if err := t.db.Save(item).Error; err != nil {
		return pubsub.None{}, err
	}
	return pubsub.None{}, t.db.Delete(item).Error
}

func (t *tx) clearHashes(pubsub.None) (int64, error) {
	res := t.db.Unscoped().Model(&dto.ItemDto{}).Where("hash_short <> ''").UpdateColumn("hash_short", "")
	return res.RowsAffected, res.Error
}

func newItemCommands(p *Proxy) itemCommands {
	return itemCommands{
		createItem:  command(p, pubsub.Frame, (*tx).createItem),
		updateItem:  command(p, pubsub.Frame, (*tx).updateItem),
		deleteItem:  command(p, pubsub.Frame, (*tx).deleteItem),
		clearHashes: command(p, pubsub.Frame, (*tx).clearHashes),
	}
}
