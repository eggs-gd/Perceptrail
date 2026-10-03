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
	l "github.com/eggs-gd/perceplib/logger"
)

// New: in — the perceived items; out — the published ones. Its steps report to
// errch.
func New(db CloserStore, values Values, in <-chan *identify.Item, out chan<- *dto.ItemDto, errch chan error, logger *l.Logger) chain.ChainProcessor {
	// keep → close: the values kept
	kept := make(chan *identify.Item)

	stage := chain.NewChainProcessor(errch)
	stage.AddStep(chain.NewDecorator(in, kept, NewKeep(values)))
	stage.AddStep(chain.NewDecorator(kept, out, NewCloser(db)))
	return stage
}
