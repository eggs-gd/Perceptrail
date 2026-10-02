// Package commit: the last stage of the import — the item published: saved Visible
// (a preview) or Waiting (none), its perceptors' values written with it.
package commit

import (
	"fmt"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// New: in — the perceived items; out — the published ones. Its step reports to errch.
func New(db model.ItemsApi, in <-chan *flow.RawItem, out chan<- *dto.ItemDto, errch chan error, logger *l.Logger) chain.ChainProcessor {
	stage := chain.NewChainProcessor(errch)
	stage.AddStep(chain.NewDecorator(in, out, NewCloser(db, logger.Named(string(app.LogPluginExifCloser)))))
	return stage
}

// Closer: the closer step's logic
type Closer struct {
	db     model.ItemsApi
	logger *l.Logger
}

func NewCloser(db model.ItemsApi, logger *l.Logger) *Closer { return &Closer{db: db, logger: logger} }

func (c *Closer) Decorate(in *flow.RawItem) (*dto.ItemDto, error) {
	if in == nil || in.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	// The end of the cheap stage: shown if there is a preview; Ready comes from the
	// expensive stage (transcode)
	in.Item.State = dto.Waiting
	if in.Item.PreviewPath != "" {
		in.Item.State = dto.Visible
	}
	item, err := c.db.UpdateItem(in.Item)
	if err != nil {
		return nil, err
	}
	// The perceptors' values go with the item: a row in each import perceptor's
	// storage — its value, or "processed, nothing found" (no GPS)
	for _, st := range plugins.Pm.ImportStores() {
		v, _ := in.StoreValues(st.Name())
		if err := st.Save(item.Guid, v); err != nil {
			return nil, fmt.Errorf("perceptor %s: %w", st.Name(), err)
		}
	}
	return item, nil
}

func (c *Closer) Stop() {}
