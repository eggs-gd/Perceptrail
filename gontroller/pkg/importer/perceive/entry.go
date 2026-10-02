// Package perceive: the perceptors over the identified item (the core's, then the
// external plugins), then the closer — the item published (Visible or Waiting),
// its perceptors' values written with it.
package perceive

import (
	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// New: in — the identified items; out — the published ones. Its steps report to
// errch.
func New(db model.Store, in <-chan *flow.RawItem, out chan<- *dto.ItemDto, errch chan error, logger *l.Logger) chain.Processor {
	return NewExifPluginProcessor(db, in, out, errch, logger)
}
