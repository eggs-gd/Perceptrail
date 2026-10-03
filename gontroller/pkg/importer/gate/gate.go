// Package gate: the third step of the import — keeps the files table (identity,
// stat, CheckTime) and lets through only the groups that need work (the model says
// which), in our format: so unchanged files never reach exiftool. It knows nothing
// of walks or deletions: those are the importer's, after a walk.
package gate

import (
	"errors"
	"time"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Store: what the gate reads and writes — the files table, and whether a group
// needs work (the model's rule)
type Store interface {
	GetFileByPath(path string) (*dto.FileDto, error)
	CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	NeedsWork(files []*dto.FileDto, key, metaHash string) (needs bool, guid string, err error)
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

// Group: what the gate yields — one whole asset that needs work, in our format: its
// files are rows of the files table (GUIDs, links), the main file first when its
// source knows it. What the source said about it rides along.
type Group struct {
	Files []*dto.FileDto
	// The item's GUID when the source knows the asset (Apple Photos: its UUID; then
	// Files[0] is the main file and is not re-ranked); "": a plain folder's group
	Key string
	// What to show first, best first (stored rows); nil: identify decides
	Show []*dto.FileDto
	// The source's own metadata (exiftool's tag names): wins over the files' EXIF;
	// MetaHash is saved with the item (the gate compares it)
	Meta     api.RawExif
	MetaHash string
	// What the asset is (dto.Kind*), when the source says it
	Kind string
}

// Gate: the gate step's logic
type Gate struct {
	db     Store
	logger *l.Logger
}

func NewGate(db Store, logger *l.Logger) *Gate {
	return &Gate{db: db, logger: logger}
}

// New: in — whole assets (the groupers', and one asked again on demand); out — the
// ones that need work
func New(db Store, logger *l.Logger, in *chain.Pipe[providers.Group], out *chain.Pipe[Group]) chain.Processor {
	return chain.Decorate(in, out, NewGate(db, logger))
}

func (g *Gate) Decorate(in providers.Group) (Group, error) {
	// Files a grouper held back are there (seen; their asset not complete yet): stamped,
	// so the deletions after the walk do not take them as gone
	if err := g.stamp(in.Held); err != nil {
		return Group{}, err
	}
	files, err := g.pass(in.Files, in.Key, in.MetaHash, in.Requested)
	if err != nil {
		return Group{}, err
	}
	return Group{Files: files, Key: in.Key, Show: stored(in.Show, files),
		Meta: in.Meta, MetaHash: in.MetaHash, Kind: in.Kind}, nil
}

// stamp: these files were seen by this walk (their rows, if any, get CheckTime)
func (g *Gate) stamp(paths []string) error {
	var rows []*dto.FileDto
	for _, p := range paths {
		if f, err := g.db.GetFileByPath(p); err == nil {
			f.CheckTime = time.Now()
			rows = append(rows, f)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := g.db.UpdateFiles(rows)
	return err
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

// pass stores the group and lets it through if it needs work (or was asked for)
func (g *Gate) pass(found []*dto.FileDto, key, metaHash string, requested bool) ([]*dto.FileDto, error) {
	if len(found) == 0 {
		return nil, chain.ErrSkippedItem
	}
	files, changed, err := g.store(found)
	if err != nil {
		return nil, err
	}
	if changed || requested || g.needsWork(files, key, metaHash) {
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
