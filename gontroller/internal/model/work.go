package model

import (
	"errors"
	"time"

	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	"github.com/eggs-gd/perceplib/api"
	"gorm.io/gorm/clause"
)

// The work queue: the expensive stage's work per item and slug ("render"). What is
// needed is derived — an item needs a slug's work when it has no row, or its row was
// done with another version or for another input (the item's fingerprint); only
// what was done, failed or taken is recorded.
//
// Not obvious:
//   - A pass goes Waiting items first (nothing shows them yet), then the shown ones
//     newest first, page by page after a cursor (a keyset on the date: the
//     (state, date) index, no sort of everything).
//   - Taken is a lease, not a mark: a crash lets it expire and the item comes back.
//     A result counts only with the lease's token: a worker that ran past its lease,
//     the item taken by another meanwhile, changes nothing.
//   - A result for an input that changed meanwhile is dropped; the item stays due.
//   - Failures back off (1 min, 10 min, 1 h, 1 day); after maxAttempts the item
//     waits for a new version or input.

// TakeArgs: items to take into a slug's work
type TakeArgs struct {
	Slug  string
	GUIDs []api.GUID
}

const (
	lease       = 15 * time.Minute
	maxAttempts = 5
)

// backoff: how long a failed item waits, by its failures in a row
var backoff = []time.Duration{time.Minute, 10 * time.Minute, time.Hour, 24 * time.Hour}

// workCommands: the queue's writes as commands
type workCommands struct {
	take   op[TakeArgs, []dto.Taken]
	finish op[dto.WorkDone, bool]
	fail   op[dto.WorkFailed, pubsub.None]
}

// Due: the next page of items that need the slug's work done with this version,
// after the cursor; an empty page and an Over cursor end the pass
func (q query) Due(slug, version string, after dto.Cursor, n int) ([]*dto.ItemDto, dto.Cursor, error) {
	now := time.Now().Unix()
	for phase := after.Phase; phase <= 1; phase++ {
		states := []dto.ItemState{dto.Waiting}
		if phase == 1 {
			states = []dto.ItemState{dto.Visible, dto.Ready}
		}
		page := q.db.Model(&dto.ItemDto{}).Select("items.*").
			Joins("LEFT JOIN work ON work.guid = items.guid AND work.slug = ?", slug).
			Where("items.state IN ?", states).
			Where(`(work.guid IS NULL OR work.version <> ? OR work.input <> items.hash_short
				OR (work.done_at = 0 AND work.next_try <= ? AND work.attempts < ?))`, version, now, maxAttempts).
			Where("(work.guid IS NULL OR work.lease_until <= ?)", now)
		if phase == after.Phase && after.ID != 0 {
			page = page.Where("(items.date < ? OR (items.date = ? AND items.id < ?))", after.Date, after.Date, after.ID)
		}
		var items []*dto.ItemDto
		if err := page.Order("items.date DESC, items.id DESC").Limit(n).Find(&items).Error; err != nil {
			return nil, after, err
		}
		if len(items) > 0 {
			last := items[len(items)-1]
			return items, dto.Cursor{Phase: phase, Date: last.Date, ID: last.ID}, nil
		}
	}
	return nil, dto.Cursor{Phase: 2}, nil
}

// Renditions: an item's renditions of the version its work was last done with,
// smallest first
func (q query) Renditions(guid api.GUID) ([]dto.RenditionDto, error) {
	rendered, err := q.renditionsOf([]api.GUID{guid})
	return rendered[guid], err
}

// Work: an item's row for a slug — done, failed (its error) or taken; ErrNotFound
// when there is none (the work was never tried)
func (q query) Work(guid api.GUID, slug string) (*dto.WorkDto, error) {
	var row dto.WorkDto
	return &row, q.db.Where("guid = ? AND slug = ?", guid, slug).First(&row).Error
}

func (p *Proxy) Take(slug string, guids []api.GUID) ([]dto.Taken, error) {
	return p.take.Do(TakeArgs{slug, guids})
}

func (p *Proxy) TakeCommand() pubsub.Command[TakeArgs, []dto.Taken] {
	return p.take
}

// Finish: false when the result was dropped (the lease lost, the item changed or
// went meanwhile)
func (p *Proxy) Finish(done dto.WorkDone) (bool, error) {
	return p.finish.Do(done)
}

func (p *Proxy) FinishCommand() pubsub.Command[dto.WorkDone, bool] {
	return p.finish
}

func (p *Proxy) Fail(failed dto.WorkFailed) error {
	_, err := p.fail.Do(failed)
	return err
}

func (p *Proxy) FailCommand() pubsub.Message[dto.WorkFailed] {
	return pubsub.MessageOf(p.fail)
}

// take: a lease on each item no one holds, under one token; the items taken
func (t *tx) take(a TakeArgs) ([]dto.Taken, error) {
	now := time.Now()
	var held []api.GUID
	err := t.db.Model(&dto.WorkDto{}).Where("slug = ? AND guid IN ? AND lease_until > ?", a.Slug, a.GUIDs, now.Unix()).
		Pluck("guid", &held).Error
	if err != nil {
		return nil, err
	}
	busy := make(map[api.GUID]bool, len(held))
	for _, g := range held {
		busy[g] = true
	}
	token := now.UnixNano()
	var taken []dto.Taken
	for _, g := range a.GUIDs {
		if busy[g] {
			continue
		}
		row := dto.WorkDto{GUID: g, Slug: a.Slug, LeaseUntil: now.Add(lease).Unix(), Lease: token}
		err := t.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "guid"}, {Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{"lease_until", "lease"}),
		}).Create(&row).Error
		if err != nil {
			return taken, err
		}
		taken = append(taken, dto.Taken{GUID: g, Lease: token})
	}
	return taken, nil
}

// finish: the work done — its row and its renditions — unless the lease was lost
// (nothing changes) or the item changed or went meanwhile (the lease goes, the item
// stays due)
func (t *tx) finish(a dto.WorkDone) (bool, error) {
	if held, err := t.holds(a.Slug, a.GUID, a.Lease); err != nil || !held {
		return false, err
	}
	item, err := t.GetItemByGUID(a.GUID)
	if errors.Is(err, ErrNotFound) || (err == nil && item.HashShort != a.Input) {
		return false, t.release(a.Slug, a.GUID)
	}
	if err != nil {
		return false, err
	}
	row := dto.WorkDto{GUID: a.GUID, Slug: a.Slug, Version: a.Version, Input: a.Input, DoneAt: time.Now().Unix()}
	if err := t.db.Save(&row).Error; err != nil {
		return false, err
	}
	if err := t.db.Where("guid = ? AND version = ?", a.GUID, a.Version).Delete(&dto.RenditionDto{}).Error; err != nil {
		return false, err
	}
	if len(a.Renditions) > 0 {
		if err := t.db.Create(&a.Renditions).Error; err != nil {
			return false, err
		}
		item.State = dto.Ready // what the browser shows is ours now
		if _, err := t.updateItem(item); err != nil {
			return false, err
		}
	}
	return true, nil
}

// fail: the failure kept, the item backs off; failures count in a row for the same
// version and input. A lease lost: nothing changes
func (t *tx) fail(a dto.WorkFailed) (pubsub.None, error) {
	if held, err := t.holds(a.Slug, a.GUID, a.Lease); err != nil || !held {
		return pubsub.None{}, err
	}
	var rows []dto.WorkDto
	if err := t.db.Where("guid = ? AND slug = ?", a.GUID, a.Slug).Limit(1).Find(&rows).Error; err != nil {
		return pubsub.None{}, err
	}
	attempts := 1
	if len(rows) == 1 && rows[0].DoneAt == 0 && rows[0].Version == a.Version && rows[0].Input == a.Input {
		attempts = rows[0].Attempts + 1
	}
	wait := backoff[min(attempts, len(backoff))-1]
	row := dto.WorkDto{GUID: a.GUID, Slug: a.Slug, Version: a.Version, Input: a.Input,
		Attempts: attempts, NextTry: time.Now().Add(wait).Unix(), Error: a.Err}
	return pubsub.None{}, t.db.Save(&row).Error
}

// release: the lease goes, nothing else changes
func (t *tx) release(slug string, guid api.GUID) error {
	return t.db.Model(&dto.WorkDto{}).Where("guid = ? AND slug = ?", guid, slug).
		Updates(map[string]any{"lease_until": 0, "lease": 0}).Error
}

// holds: the row is still this lease's (0: never taken, no one took it since)
func (t *tx) holds(slug string, guid api.GUID, lease int64) (bool, error) {
	var rows []dto.WorkDto
	if err := t.db.Where("guid = ? AND slug = ?", guid, slug).Limit(1).Find(&rows).Error; err != nil {
		return false, err
	}
	return len(rows) == 0 || rows[0].Lease == lease, nil
}

func newWorkCommands(p *Proxy) workCommands {
	return workCommands{
		take:   command(p, pubsub.Frame, (*tx).take),
		finish: command(p, pubsub.Now, (*tx).finish),
		fail:   command(p, pubsub.Frame, (*tx).fail),
	}
}

// rendered: the item has renditions made for its fingerprint as it is now
func (q query) rendered(item *dto.ItemDto) bool {
	var n int64
	q.db.Model(&dto.RenditionDto{}).
		Joins("JOIN work ON work.guid = renditions.guid AND work.version = renditions.version").
		Where("renditions.guid = ? AND work.done_at > 0 AND work.input = ?", item.GUID, item.HashShort).
		Count(&n)
	return n > 0
}

// renditionsOf: the renditions of these items, each of the version its work was last
// done with, smallest first
func (q query) renditionsOf(guids []api.GUID) (map[api.GUID][]dto.RenditionDto, error) {
	var rows []dto.RenditionDto
	err := q.db.Model(&dto.RenditionDto{}).Select("renditions.*").
		Joins("JOIN work ON work.guid = renditions.guid AND work.version = renditions.version AND work.done_at > 0").
		Where("renditions.guid IN ?", guids).Order("renditions.size").Find(&rows).Error
	out := map[api.GUID][]dto.RenditionDto{}
	for _, r := range rows {
		out[r.GUID] = append(out[r.GUID], r)
	}
	return out, err
}
