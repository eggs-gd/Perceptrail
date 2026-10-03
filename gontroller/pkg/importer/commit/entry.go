// Package commit: the last step of the import — the item published: the model sets
// its state (Visible with a preview, Waiting without) and saves it.
package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// New: in — the perceived items (their values kept); out — the published ones
func New(db CloserStore, in *chain.Pipe[*identify.Item], out *chain.Pipe[*dto.ItemDto]) chain.Processor {
	return chain.Decorate(in, out, NewCloser(db))
}
