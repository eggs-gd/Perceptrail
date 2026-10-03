// Package commit: the last step of the import, the chain's end — the item
// published: the model sets its state (Visible with a preview, Waiting without) and
// saves it.
package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"

	"github.com/eggs-gd/perceplib/chain"
)

// New: in — the perceived items (their values kept)
func New(db CloserStore, in *chain.Pipe[*identify.Item]) chain.Processor {
	return chain.End(in, NewCloser(db))
}
