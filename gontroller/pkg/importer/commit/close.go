package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model/dto"
)

// CloserStore: what the closer writes — the item published (the model decides its
// state)
type CloserStore interface {
	Publish(item *dto.ItemDto) (*dto.ItemDto, error)
}

// Closer: the close step's logic — the item published after the cheap stage
type Closer struct {
	db        CloserStore
	published func(guid string)
}

func NewCloser(db CloserStore, published func(guid string)) *Closer {
	return &Closer{db: db, published: published}
}

func (c *Closer) Consume(in *identify.Item) error {
	if in == nil || in.Item == nil {
		return nil
	}
	item, err := c.db.Publish(in.Item)
	if err != nil {
		return err
	}
	if c.published != nil {
		c.published(item.Guid)
	}
	return nil
}
