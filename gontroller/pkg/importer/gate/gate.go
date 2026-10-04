// Package gate: the third step of the import — lets through only the assets that
// need work (the model says which): so unchanged files never reach exiftool. A file the walk says is gone (and its provider let through) is the
// model's to delete here.
package gate

import (
	"slices"

	"perceptrail/gontroller/pkg/model/dto"

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

// Gate: the gate step's logic
type Gate struct {
	db     Store
	logger *l.Logger
}

// New: in — whole assets (the groupers'); out — the ones that need work
func New(db Store, logger *l.Logger, in <-chan dto.Asset, out chan<- dto.Asset) chain.Processor {
	return chain.NewDecorator(in, out, &Gate{db: db, logger: logger})
}

func (g *Gate) Decorate(in dto.Asset) (dto.Asset, error) {
	files, err := g.gone(in.Files)
	if err != nil || len(files) == 0 {
		return dto.Asset{}, skipOr(err)
	}
	changed := slices.ContainsFunc(files, func(f *dto.FileDto) bool { return f.Changed })
	if changed {
		// The walk stored the stat; a role the grouper gave is stored here
		if _, err := g.db.UpdateFiles(files); err != nil {
			return dto.Asset{}, err
		}
	}
	if !changed && !g.needsWork(files, in.Key, in.MetaHash) {
		return dto.Asset{}, chain.ErrSkippedItem
	}
	in.Files = files
	return in, nil
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
