// Package commit: the last step of the import, the chain's end — the item
// published: the model sets its state (Visible with a preview, Waiting without) and
// saves it.
package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// Store: what commit writes — the item published (the model decides its state)
type Store interface {
	Publish(item *dto.ItemDto) (*dto.ItemDto, error)
}

// New: in — the perceived items (their values kept)
func New(db Store, in <-chan *identify.Item) chain.Processor {
	return chain.NewEnd(in, publish{db})
}

type publish struct{ db Store }

func (p publish) Consume(in *identify.Item) error {
	if in == nil || in.Item == nil {
		return nil
	}
	_, err := p.db.Publish(in.Item)
	return err
}
