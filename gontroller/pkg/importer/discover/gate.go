package discover

import (
	"errors"
	"time"

	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// GateStore: what the gate reads and writes — the files table, and an item's state
// (does its group need work)
type GateStore interface {
	GetFileByPath(path string) (*dto.FileDto, error)
	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	GetItemByGuid(guid string) (*dto.ItemDto, error)
}

// Store: what discover reads and writes — the gate's and the deletions' needs
type Store interface {
	GateStore
	SweepStore
}

// Unprocessed: whether a perceptor still has to process an item (its schema is new
// or changed) — the group goes through the import once more
type Unprocessed interface {
	Unprocessed(guid string) bool
}

// Perceptors: what discover asks of the perceptors — the gate and the deletions
type Perceptors interface {
	Unprocessed
	Pruner
}

// files gate: keeps the files table (identity, stat, CheckTime) and lets through
// only groups that need work — so unchanged files never reach exiftool. After the
// end-of-walk marker from every grouper it derives deletions.
type Gate struct {
	db         GateStore
	perceptors Unprocessed
	sweep      sweep
	logger     *l.Logger
	branches   int // markers to wait for
	markers    int
	held       []string // files the groupers held back in this walk: not gone
	progress   *flow.Progress
	// dropped hears of a keyed group the gate let not through (nothing to do): one
	// asset processed again on demand (importer Refresh) answers at once
	dropped func(key string)
}

// NewGate: the gate step's logic. branches is the number of groupers that send an
// end-of-walk marker; dropped hears of a keyed group let not through (nil: nobody).
func NewGate(db Store, perceptors Perceptors, branches int, progress *flow.Progress, dropped func(key string), logger *l.Logger) *Gate {
	return &Gate{
		db: db, perceptors: perceptors, sweep: sweep{db: db, perceptors: perceptors, logger: logger},
		logger: logger, branches: branches, progress: progress, dropped: dropped,
	}
}

func (g *Gate) Decorate(in flow.FileGroup) (flow.FileGroup, error) {
	// A grouper's last group comes with its end-of-walk marker: the group first
	out, err := g.pass(in.Files, in.Key, in.MetaHash)
	if err == nil {
		out.Key, out.Show = in.Key, stored(in.Show, out.Files)
		out.Meta, out.MetaHash, out.Kind = in.Meta, in.MetaHash, in.Kind
		g.progress.Passed()
	} else if in.Key != "" && in.Files != nil && g.dropped != nil {
		g.dropped(in.Key)
	}
	if in.Done != nil {
		g.held = append(g.held, in.Held...)
		g.markers++
		if g.markers == g.branches { // every grouper has flushed: all files are stamped
			g.markers = 0
			g.sweep.finalizeWalk(*in.Done, g.held)
			g.held = nil
			g.progress.WalkGated()
		}
	}
	return out, err
}

// stored maps the grouper's files to the stored rows of the group (by path): the
// later steps fill the rows (MimeType, links)
func stored(files, rows []*dto.FileDto) []*dto.FileDto {
	if files == nil {
		return nil
	}
	byPath := make(map[string]*dto.FileDto, len(rows))
	for _, r := range rows {
		byPath[r.Path] = r
	}
	out := make([]*dto.FileDto, 0, len(files))
	for _, f := range files {
		if r, ok := byPath[f.Path]; ok {
			out = append(out, r)
		}
	}
	return out
}

// pass stores the group and lets it through if it needs work
func (g *Gate) pass(found []*dto.FileDto, key, metaHash string) (flow.FileGroup, error) {
	if len(found) == 0 {
		return flow.FileGroup{}, chain.ErrSkippedItem
	}
	files, changed, err := g.store(found)
	if err != nil {
		return flow.FileGroup{}, err
	}
	if changed || g.needsProcessing(files, key, metaHash) {
		return flow.FileGroup{Files: files}, nil
	}
	return flow.FileGroup{}, chain.ErrSkippedItem
}

func (g *Gate) Stop() {}

// store finds or creates the rows of the group (found: files with their stat only),
// refreshes their stat and stamps them as seen. changed: a new file, or size/mtime differ.
func (g *Gate) store(found []*dto.FileDto) ([]*dto.FileDto, bool, error) {
	now := time.Now()
	changed := false
	files := make([]*dto.FileDto, 0, len(found))

	for _, fe := range found {
		e := fe.ItemEntry
		f, err := g.db.GetFileByPath(e.Path)
		switch {
		case errors.Is(err, model.ErrNotFound):
			if f, err = g.db.CreateFile(e); err != nil {
				return nil, false, err
			}
			changed = true
		case err != nil:
			return nil, false, err
		case !f.ModTime.Equal(e.ModTime) || f.Size != e.Size:
			// Store the fresh stat, or every walk sees the file as changed again
			// (and the short hash would use the stale size)
			f.Size, f.ModTime = e.Size, e.ModTime
			changed = true
		}
		// The source's grouper knows the role (Apple): a new role is new work
		if fe.Role != "" && f.Role != fe.Role {
			f.Role = fe.Role
			changed = true
		}
		f.CheckTime = now
		files = append(files, f)
	}

	if _, err := g.db.UpdateFiles(files); err != nil {
		return nil, false, err
	}
	return files, changed, nil
}

// needsProcessing: nothing changed on disk, but the group is not done — a file
// was never linked or is linked outside the group (its main file is gone: a RAW
// deleted, its JPEG left), or the item is missing or not Ready (new, Dirty,
// interrupted). Groups that are known not to be media stay ignored.
func (g *Gate) needsProcessing(files []*dto.FileDto, key, metaHash string) bool {
	inGroup := map[string]bool{key: key != ""}
	for _, f := range files {
		inGroup[f.GUID] = true
	}
	main := ""
	for _, f := range files {
		switch {
		case f.LinkedTo == "":
			return true
		case !f.IsIgnored() && f.Role == "":
			return true // from before roles existed: classified once more
		case f.IsIgnored():
		case !inGroup[f.LinkedTo]:
			return true
		case main == "":
			main = f.LinkedTo
		}
	}
	if main == "" {
		return false // the whole group is ignored
	}
	item, err := g.db.GetItemByGuid(main)
	if err != nil {
		return errors.Is(err, model.ErrNotFound)
	}
	// The source's metadata changed (a date corrected in Photos), the files did not;
	// or a perceptor has not processed it (new, or its schema changed)
	return !cheapStageDone(item) || item.MetaHash != metaHash || g.perceptors.Unprocessed(item.Guid)
}

// cheapStageDone: the item went through the cheap stage (Visible, Waiting) or is
// fully done (Ready). An item shown without a preview is from before the cheap
// stage existed: it goes through once more.
func cheapStageDone(item *dto.ItemDto) bool {
	switch item.State {
	case dto.Visible, dto.Ready:
		return item.PreviewPath != ""
	case dto.Waiting:
		return true
	}
	return false
}
