// Package commit: the last step of the import, the chain's end — the item
// published: the model sets its state (Visible with a preview, Waiting without) and
// saves it; who waits for it hears (published).
package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"

	"github.com/eggs-gd/perceplib/chain"
)

// New: in — the perceived items (their values kept); published hears every item
// published (by GUID; nil: nobody)
func New(db CloserStore, published func(guid string), in *chain.Pipe[*identify.Item]) chain.Processor {
	return chain.End(in, NewCloser(db, published))
}
