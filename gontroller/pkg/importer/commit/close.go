package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// CloserStore: what the closer writes — the item published (the model decides its
// state)
type CloserStore interface {
	Publish(item *dto.ItemDto) (*dto.ItemDto, error)
}

// Closer: the close step's logic — the item published after the cheap stage
type Closer struct {
	db CloserStore
}

func NewCloser(db CloserStore) *Closer { return &Closer{db: db} }

func (c *Closer) Decorate(in *identify.Item) (*dto.ItemDto, error) {
	if in == nil || in.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	return c.db.Publish(in.Item)
}

func (c *Closer) Stop() {}
