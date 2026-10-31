// Package gate: the third step of the import — lets through only the assets that
// need work (the model says which): so unchanged files never reach exiftool. The
// files gone for the library (the asset's Missing: the walk's, or its provider's)
// are the model's to apply here.
package gate

import (
	"slices"

	"perceptrail/gontroller/internal/model/dto"

	chain "github.com/eggs-gd/go-chain"
	"github.com/eggs-gd/perceplib/api"

	l "github.com/eggs-gd/go-zap-decor"
)

// Store: what the gate asks the model — whether a group needs work, what a file gone
// means
type Store interface {
	NeedsWork(files []*dto.FileDto, key api.GUID, metaHash string) (needs bool, guid api.GUID, err error)
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
	if err := g.gone(in.Missing); err != nil {
		return dto.Asset{}, err
	}
	files := in.Files
	if len(files) == 0 {
		return dto.Asset{}, chain.ErrSkippedItem
	}
	// Changed: new, its stat or the role its grouper gave (stored later, with the
	// sizes; a group that fails before shows the change again next walk)
	changed := slices.ContainsFunc(files, func(f *dto.FileDto) bool { return f.Changed })
	if !changed && !g.needsWork(files, in.Key, in.MetaHash) {
		return dto.Asset{}, chain.ErrSkippedItem
	}
	in.Missing = nil
	return in, nil
}

// gone: the files gone for the library go by the model's rules (a main file's item
// deleted, a sidecar's item processed again)
func (g *Gate) gone(missing []*dto.FileDto) error {
	if len(missing) == 0 {
		return nil
	}
	deleted, dirty, err := g.db.Gone(missing)
	if err != nil {
		return err
	}
	g.logger.Debug("Gone", l.String("file", missing[0].Path), l.Int("items", deleted), l.Int("dirty", dirty))
	return nil
}

// needsWork: nothing changed on disk — the model says whether the group still needs
// work
func (g *Gate) needsWork(files []*dto.FileDto, key api.GUID, metaHash string) bool {
	needs, _, err := g.db.NeedsWork(files, key, metaHash)
	if err != nil {
		g.logger.Error("Gate: can't tell whether a group needs work", l.String("file", files[0].Path), l.Error(err))
		return false
	}
	return needs
}
