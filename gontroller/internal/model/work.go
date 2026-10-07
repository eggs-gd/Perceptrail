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
//   - A result for an input that changed meanwhile is dropped; the item stays due.
//   - Failures back off (1 min, 10 min, 1 h, 1 day); after maxAttempts the item
//     waits for a new version or input.

// Cursor: where a pass over the due items is; the zero value starts one
type Cursor struct {
	phase int // 0: Waiting items; 1: the shown ones (Visible, Ready); 2: the pass is over
	date  time.Time
	id    uint // the last item of the page; 0: the phase starts
}

// TakeArgs: items to take into a slug's work
type TakeArgs struct {
	Slug  string
	GUIDs []api.GUID
}

// FinishArgs: a slug's work done for an item — with what, for which input, what it
// made
type FinishArgs struct {
	Slug       string
	GUID       api.GUID
	Version    string
	Input      string
	Renditions []dto.RenditionDto
}

// FailArgs: a slug's work failed for an item
type FailArgs struct {
	Slug    string
	GUID    api.GUID
	Version string
	Input   string
	Err     string
}

const (
	lease       = 15 * time.Minute
	maxAttempts = 5
)

// backoff: how long a failed item waits, by its failures in a row
var backoff = []time.Duration{time.Minute, 10 * time.Minute, time.Hour, 24 * time.Hour}

// workCommands: the queue's writes as commands
type workCommands struct {
	take   op[TakeArgs, []api.GUID]
	finish op[FinishArgs, bool]
	fail   op[FailArgs, pubsub.None]
}

// Over: the pass has no more pages
func (c Cursor) Over() bool { return c.phase > 1 }

// Due: the next page of items that need the slug's work done with this version,
// after the cursor; an empty page and an Over cursor end the pass
func (q query) Due(slug, version string, after Cursor, n int) ([]*dto.ItemDto, Cursor, error) {
	now := time.Now().Unix()
	for phase := after.phase; phase <= 1; phase++ {
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
		if phase == after.phase && after.id != 0 {
			page = page.Where("(items.date < ? OR (items.date = ? AND items.id < ?))", after.date, after.date, after.id)
		}
		var items []*dto.ItemDto
		if err := page.Order("items.date DESC, items.id DESC").Limit(n).Find(&items).Error; err != nil {
			return nil, after, err
		}
		if len(items) > 0 {
			last := items[len(items)-1]
			return items, Cursor{phase, last.Date, last.ID}, nil
		}
	}
	return nil, Cursor{phase: 2}, nil
}

func (p *Proxy) Take(slug string, guids []api.GUID) ([]api.GUID, error) {
	return p.take.Do(TakeArgs{slug, guids})
}

func (p *Proxy) TakeCommand() pubsub.Command[TakeArgs, []api.GUID] {
	return p.take
}

// Finish: false when the result was dropped (the item changed or went meanwhile)
func (p *Proxy) Finish(args FinishArgs) (bool, error) {
	return p.finish.Do(args)
}

func (p *Proxy) FinishCommand() pubsub.Command[FinishArgs, bool] {
	return p.finish
}

func (p *Proxy) Fail(args FailArgs) error {
	_, err := p.fail.Do(args)
	return err
}

func (p *Proxy) FailCommand() pubsub.Message[FailArgs] {
	return pubsub.MessageOf(p.fail)
}

// take: a lease on each item no one holds; the items taken
func (t *tx) take(a TakeArgs) ([]api.GUID, error) {
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
	var taken []api.GUID
	for _, g := range a.GUIDs {
		if busy[g] {
			continue
		}
		row := dto.WorkDto{GUID: g, Slug: a.Slug, LeaseUntil: now.Add(lease).Unix()}
		err := t.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "guid"}, {Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{"lease_until"}),
		}).Create(&row).Error
		if err != nil {
			return taken, err
		}
		taken = append(taken, g)
	}
	return taken, nil
}

// finish: the work done — its row and its renditions — unless the item changed or
// went meanwhile: then the lease goes and the item stays due
func (t *tx) finish(a FinishArgs) (bool, error) {
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
	}
	return true, nil
}

// fail: the failure kept, the item back off; failures count in a row for the same
// version and input
func (t *tx) fail(a FailArgs) (pubsub.None, error) {
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
	return t.db.Model(&dto.WorkDto{}).Where("guid = ? AND slug = ?", guid, slug).Update("lease_until", 0).Error
}

func newWorkCommands(p *Proxy) workCommands {
	return workCommands{
		take:   command(p, pubsub.Frame, (*tx).take),
		finish: command(p, pubsub.Now, (*tx).finish),
		fail:   command(p, pubsub.Frame, (*tx).fail),
	}
}
