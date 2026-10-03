// Package gate: the third step of the import — lets through only the groups that
// need work (the model says which), in our format: so unchanged files never reach
// exiftool. A file the walk says is gone (and its provider let through) is the
// model's to delete here.
package gate

import (
	"slices"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// Store: what the gate asks the model — whether a group needs work, what a file gone
// means; the rows a grouper gave a new role
type Store interface {
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	NeedsWork(files []*dto.FileDto, key, metaHash string) (needs bool, guid string, err error)
	Gone(files []*dto.FileDto) (deleted, dirty int, err error)
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

// New: in — whole assets (the groupers'); out — the ones that need work
func New(db Store, logger *l.Logger, in *chain.Pipe[providers.Asset], out *chain.Pipe[Group]) chain.Processor {
	return chain.Decorate(in, out, NewGate(db, logger))
}

func (g *Gate) Decorate(in providers.Asset) (Group, error) {
	files, err := g.gone(in.Files)
	if err != nil || len(files) == 0 {
		return Group{}, skipOr(err)
	}
	changed := slices.ContainsFunc(files, func(f *dto.FileDto) bool { return f.Changed })
	if changed {
		// The walk stored the stat; a role the grouper gave is stored here
		if _, err := g.db.UpdateFiles(files); err != nil {
			return Group{}, err
		}
	}
	if !changed && !g.needsWork(files, in.Key, in.MetaHash) {
		return Group{}, chain.ErrSkippedItem
	}
	return Group{Files: files, Key: in.Key, Show: in.Show, Meta: in.Meta, MetaHash: in.MetaHash, Kind: in.Kind}, nil
}

// gone: the files the walk says are gone go by the model's rules (a main file's item
// deleted, a sidecar's item processed again); the rest
func (g *Gate) gone(files []*dto.FileDto) ([]*dto.FileDto, error) {
	var gone, rest []*dto.FileDto
	for _, f := range files {
		if f.Gone {
			gone = append(gone, f)
		} else {
			rest = append(rest, f)
		}
	}
	if len(gone) > 0 {
		deleted, dirty, err := g.db.Gone(gone)
		if err != nil {
			return nil, err
		}
		g.logger.Debug("Gone", l.String("file", gone[0].Path), l.Int("items", deleted), l.Int("dirty", dirty))
	}
	return rest, nil
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

func skipOr(err error) error {
	if err != nil {
		return err
	}
	return chain.ErrSkippedItem
}
