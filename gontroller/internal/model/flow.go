package model

import (
	"errors"

	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	"github.com/eggs-gd/perceplib/api"
)

// An item's flow through its states — the library's own rules: when a group needs
// work, what a file gone means, when an item is shown. Every chain that touches
// items (the import, an asset again on demand, later the API providers and the
// maintenance) goes by the same ones; a step gathers the facts (stat, exif, kinds,
// fingerprint), the model decides and keeps.

// GoneResult: what gone did — items deleted, items made Dirty
type GoneResult struct {
	Deleted, Dirty int
}

// flowCommands: the flow's writes as commands
type flowCommands struct {
	gone          op[[]*dto.FileDto, GoneResult]
	ignore        op[[]*dto.FileDto, pubsub.None]
	publish       op[*dto.ItemDto, *dto.ItemDto]
	markRework    op[[]api.GUID, int64]
	markAllRework op[pubsub.None, int64]
}

// NeedsWork: the group's files are stored and unchanged on disk — does it still
// need work? Yes when a file was never linked or is linked outside the group (its
// main file is gone: a RAW deleted, its JPEG left), a file has no role (from before
// roles existed), the item is missing or not through the cheap stage, it has no
// fingerprint (the fingerprint changed: it gets the new one), or the source's
// metadata changed while the files did not (metaHash), or it is marked for rework
// (MarkRework). guid: the group's item; "" when the whole group is ignored (not media,
// broken).
func (q query) NeedsWork(files []*dto.FileDto, key api.GUID, metaHash string) (needs bool, guid api.GUID, err error) {
	inGroup := map[api.GUID]bool{key: key != ""}
	for _, f := range files {
		inGroup[f.GUID] = true
	}
	for _, f := range files {
		switch {
		case f.LinkedTo == "":
			return true, "", nil
		case !f.IsIgnored() && f.Role == "":
			return true, "", nil
		case f.IsIgnored():
		case !inGroup[f.LinkedTo]:
			return true, "", nil
		case guid == "":
			guid = f.LinkedTo
		}
	}
	if guid == "" {
		return false, "", nil
	}
	item, err := q.GetItemByGUID(guid)
	if errors.Is(err, ErrNotFound) {
		return true, guid, nil
	}
	if err != nil {
		return false, guid, err
	}
	return !q.cheapStageDone(item) || item.HashShort == "" || item.MetaHash != metaHash || item.Rework, guid, nil
}

// Unshown: the items nothing can show yet (no file the browser shows, no preview:
// Waiting) — what a library that draws renditions itself may fill
func (q query) Unshown() ([]*dto.ItemDto, error) {
	return q.GetItemsInStates(dto.Waiting)
}

func (p *Proxy) Gone(files []*dto.FileDto) (deleted, dirty int, err error) {
	r, err := p.gone.Do(files)
	return r.Deleted, r.Dirty, err
}

func (p *Proxy) GoneCommand() pubsub.Command[[]*dto.FileDto, GoneResult] {
	return p.gone
}

func (p *Proxy) Ignore(files []*dto.FileDto) error {
	_, err := p.ignore.Do(files)
	return err
}

func (p *Proxy) IgnoreCommand() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.ignore)
}

func (p *Proxy) Publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	return p.publish.Do(item)
}

func (p *Proxy) PublishCommand() pubsub.Command[*dto.ItemDto, *dto.ItemDto] {
	return p.publish
}

func (p *Proxy) MarkRework(guids []api.GUID) (int64, error) {
	return p.markRework.Do(guids)
}

func (p *Proxy) MarkReworkCommand() pubsub.Command[[]api.GUID, int64] {
	return p.markRework
}

// MarkAllRework sends every item through the cheap stage once more. It is for
// identify migrations whose output changed without a file change.
func (p *Proxy) MarkAllRework() (int64, error) {
	return p.markAllRework.Do(pubsub.None{})
}

func (p *Proxy) MarkAllReworkCommand() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.markAllRework)
}

// cheapStageDone: the item went through the cheap stage and got the client-facing
// placeholder color. An old item without one goes through once more, even when it
// already has renditions.
func (q query) cheapStageDone(item *dto.ItemDto) bool {
	switch item.State {
	case dto.Visible:
		return previewReady(item)
	case dto.Ready:
		return item.PreviewColor != "" && (item.PreviewPath != "" || q.rendered(item))
	case dto.Waiting:
		return item.PreviewColor != ""
	}
	return false
}

func previewReady(item *dto.ItemDto) bool {
	if item.PreviewPath == "" {
		return false
	}
	return item.PreviewColor != ""
}

// gone: these files are gone for the library (the walk found them missing, or
// their provider says so).
// A main file gone: its item is deleted; a sidecar gone: its item is Dirty (processed
// again); an item with no files left is gone too (a keyed asset: every file is
// "linked", none is "main" by its own GUID). The files' rows go.
func (t *tx) gone(files []*dto.FileDto) (GoneResult, error) {
	var deleted, dirty int
	for _, f := range files {
		switch {
		case f.IsIgnored() || f.LinkedTo == "":
		case f.LinkedTo == f.GUID: // main file: the item is gone
			item, err := t.GetItemByGUID(f.GUID)
			if err != nil {
				continue // never became an item, or already deleted
			}
			if _, err := t.deleteItem(item); err != nil {
				return GoneResult{deleted, dirty}, err
			}
			deleted++
		default: // sidecar: its item must be processed again
			item, err := t.GetItemByGUID(f.LinkedTo)
			if err != nil {
				continue
			}
			item.State = dto.Dirty
			if _, err := t.updateItem(item); err != nil {
				return GoneResult{deleted, dirty}, err
			}
			dirty++
		}
	}
	if _, err := t.deleteFiles(files); err != nil {
		return GoneResult{deleted, dirty}, err
	}
	for _, f := range files {
		if f.LinkedTo == "" || f.IsIgnored() {
			continue
		}
		if n, err := t.CountLinkedFiles(f.LinkedTo); err == nil && n == 0 {
			if item, err := t.GetItemByGUID(f.LinkedTo); err == nil {
				if _, err := t.deleteItem(item); err == nil {
					deleted++
				}
			}
		}
	}
	return GoneResult{deleted, dirty}, nil
}

// ignore: the group is not an item (not media, or its main file is broken): its
// files are remembered as ignored — the gate skips them until a file changes — and
// an item its main file used to be (it got corrupted) goes
func (t *tx) ignore(files []*dto.FileDto) (pubsub.None, error) {
	if item, err := t.GetItemByGUID(files[0].GUID); err == nil {
		if _, err := t.deleteItem(item); err != nil {
			return pubsub.None{}, err
		}
	}
	for _, f := range files {
		f.SetIgnored()
	}
	_, err := t.updateFiles(files)
	return pubsub.None{}, err
}

// publish: the item at the end of the import's cheap stage — Ready when its
// renditions are there already (made for this fingerprint), else Visible when it has
// something the browser shows (a preview), else Waiting (the expensive stage later)
func (t *tx) publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	item.Rework = false
	switch {
	case t.rendered(item):
		item.State = dto.Ready
	case item.PreviewPath != "":
		item.State = dto.Visible
	default:
		item.State = dto.Waiting
	}
	if _, err := t.updateItem(item); err != nil {
		return item, err
	}
	emit(t, &t.events.published, dto.ItemPublished{GUID: item.GUID, State: item.State})
	return item, nil
}

// markRework: these items are processed again on the next walk (NeedsWork), their
// files unchanged — e.g. a perceptor has no row for them, a library made a file of
// one local; publishing clears the mark. Not a change the client sees: updated_at
// stays (the client's delta would bring the item back in its old state)
func (t *tx) markRework(guids []api.GUID) (int64, error) {
	var n int64
	for start := 0; start < len(guids); start += 500 { // under SQLite's variable limit
		page := guids[start:min(start+500, len(guids))]
		res := t.db.Model(&dto.ItemDto{}).Where("guid IN ?", page).UpdateColumn("rework", true)
		if res.Error != nil {
			return n, res.Error
		}
		n += res.RowsAffected
	}
	return n, nil
}

func (t *tx) markAllRework(pubsub.None) (int64, error) {
	res := t.db.Model(&dto.ItemDto{}).Where("rework = ?", false).UpdateColumn("rework", true)
	return res.RowsAffected, res.Error
}

func newFlowCommands(p *Proxy) flowCommands {
	return flowCommands{
		gone:          command(p, pubsub.Frame, (*tx).gone),
		ignore:        command(p, pubsub.Frame, (*tx).ignore),
		publish:       command(p, pubsub.Frame, (*tx).publish),
		markRework:    command(p, pubsub.Frame, (*tx).markRework),
		markAllRework: command(p, pubsub.Frame, (*tx).markAllRework),
	}
}
