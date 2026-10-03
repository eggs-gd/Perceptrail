// Package commit: the last stage of the import — the item published.
//
//	keep (the perceptors' values) → close (the item: Visible or Waiting)
//
// The values go first: an item published without them would be taken as done; a
// crash between the two leaves an item that is not, and the next walk sends it again.
package commit

import (
	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

// New: in — the perceived items; out — the published ones. Its errors go to the
// chain it runs in.
func New(db CloserStore, values Values, in *chain.Pipe[*identify.Item], out *chain.Pipe[*dto.ItemDto]) chain.Processor {
	// keep → close: the values kept
	kept := chain.NewPipe[*identify.Item](0)

	stage := chain.New(nil)
	stage.AddStep(chain.Decorate(in, kept, NewKeep(values)))
	stage.AddStep(chain.Decorate(kept, out, NewCloser(db)))
	return stage
}
