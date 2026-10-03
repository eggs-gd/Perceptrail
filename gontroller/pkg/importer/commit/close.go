package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// CloserStore: what the closer writes — the item, in its final state
type CloserStore interface {
	UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error)
}

// Closer: the close step's logic — the item saved in its state after the cheap stage
type Closer struct {
	db CloserStore
}

func NewCloser(db CloserStore) *Closer { return &Closer{db: db} }

func (c *Closer) Decorate(in *identify.Item) (*dto.ItemDto, error) {
	if in == nil || in.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	// The end of the cheap stage: shown if there is a preview; Ready comes from the
	// expensive stage (transcode)
	in.Item.State = dto.Waiting
	if in.Item.PreviewPath != "" {
		in.Item.State = dto.Visible
	}
	return c.db.UpdateItem(in.Item)
}

func (c *Closer) Stop() {}
