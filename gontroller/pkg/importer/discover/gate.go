package discover

import (
	"errors"
	"time"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// GateStore: what the gate reads and writes — the files table, and whether a group
// needs work (the model's rule)
type GateStore interface {
	GetFileByPath(path string) (*dto.FileDto, error)
	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	NeedsWork(files []*dto.FileDto, key, metaHash string) (needs bool, guid string, err error)
}

// Store: what discover reads and writes — the gate's and the deletions' needs
type Store interface {
	GateStore
	SweepStore
}

// needsWork: nothing changed on disk — the model says whether the group still needs
// work
func (g *Gate) needsWork(files []*dto.FileDto, key, metaHash string) bool {
	needs, _, err := g.db.NeedsWork(files, key, metaHash)
	if err != nil {
		g.logger.Error("Gate: can't tell whether a group needs work", l.String("file", files[0].Path), l.Error(err))
		return false
	}
	return needs
}

// files gate: keeps the files table (identity, stat, CheckTime) and lets through
// only groups that need work — so unchanged files never reach exiftool. On the walk's
// flush (once every grouper has flushed) it derives deletions.
type Gate struct {
	db       GateStore
	sweep    sweep
	logger   *l.Logger
	held     []string  // files the groupers held back in this walk: not gone
	progress *Progress // the walk the deletions are derived from
	// dropped hears of a keyed group the gate let not through (nothing to do): one
	// asset processed again on demand (importer Refresh) answers at once
	dropped func(key string)
}

// NewGate: the gate step's logic; progress gives the walk the deletions are
// derived from
func NewGate(deps Deps, progress *Progress) *Gate {
	return &Gate{
		db: deps.DB, logger: deps.Logger, dropped: deps.Dropped,
		sweep:    sweep{db: deps.DB, logger: deps.Logger},
		progress: progress,
	}
}

func (g *Gate) Decorate(in providers.Group) (Group, error) {
	g.held = append(g.held, in.Held...)
	files, err := g.pass(in.Files, in.Key, in.MetaHash)
	if err != nil {
		if in.Key != "" && in.Files != nil && g.dropped != nil {
			g.dropped(in.Key)
		}
		return Group{}, err
	}
	return Group{Files: files, Key: in.Key, Show: stored(in.Show, files),
		Meta: in.Meta, MetaHash: in.MetaHash, Kind: in.Kind}, nil
}

// Flush: every grouper has flushed (the chain's barrier) — all files of the walk are
// stamped: the deletions
func (g *Gate) Flush() ([]Group, error) {
	g.sweep.finalizeWalk(g.progress.Last(), g.held)
	g.held = nil
	return nil, nil
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
func (g *Gate) pass(found []*dto.FileDto, key, metaHash string) ([]*dto.FileDto, error) {
	if len(found) == 0 {
		return nil, chain.ErrSkippedItem
	}
	files, changed, err := g.store(found)
	if err != nil {
		return nil, err
	}
	if changed || g.needsWork(files, key, metaHash) {
		return files, nil
	}
	return nil, chain.ErrSkippedItem
}

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
