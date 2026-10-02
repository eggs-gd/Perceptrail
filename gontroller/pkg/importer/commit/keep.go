package commit

import (
	"perceptrail/gontroller/pkg/importer/flow"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
)

// Values: where the perceptors' values are kept — a row in each import perceptor's
// storage for the item; values gives a storage's value
type Values interface {
	SaveValues(guid string, values func(store string) (api.Values, bool)) error
}

// Keep: the keep step's logic — the values the perceptors put on the item, kept
type Keep struct {
	values Values
}

func NewKeep(values Values) *Keep { return &Keep{values: values} }

func (k *Keep) Decorate(it *flow.RawItem) (*flow.RawItem, error) {
	if it.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	if err := k.values.SaveValues(it.Item.Guid, it.StoreValues); err != nil {
		return nil, err
	}
	return it, nil
}

func (k *Keep) Stop() {}
