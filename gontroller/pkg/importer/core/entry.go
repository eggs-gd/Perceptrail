// Package core: the third stage of the import — the core perceptors (built in: the
// date, the size, …) over the identified item. They write into it
// (exif_core.RawItemRW): what they find goes into the item and their storages.
//
//	open → each core perceptor, in the plugin manager's order → release
package core

import (
	"fmt"

	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// New: in — the identified items; out — the same, perceived by the core. Its steps
// report to errch.
func New(in <-chan *flow.RawItem, out chan<- *flow.RawItem, errch chan error, logger *l.Logger) chain.ChainProcessor {
	stage := chain.NewChainProcessor(errch)

	// Every step reads the previous step's channel: a perceptor that gets no step must
	// not get a channel either, otherwise the chain stalls on the unread one
	prev := make(chan exif_core.RawItemRW, 1)
	stage.AddStep(chain.NewDecorator(in, prev, open{}))
	for _, plugin := range plugins.Pm.GetPlugins() {
		p, ok := plugin.(exif_core.ExifCorePerceptor)
		if !ok || plugin.DataProvider() != api.ExifDataProvider {
			continue // an external one: the plugins stage
		}
		next := make(chan exif_core.RawItemRW, 1)
		proc := p.NewProcessor(prev, next, logger.Named(plugin.Name()))
		if proc == nil {
			logger.Error("Core perceptor returned no processor, skipped", l.String("plugin", plugin.Name()))
			continue
		}
		stage.AddStep(proc)
		prev = next
	}
	stage.AddStep(chain.NewDecorator(prev, out, release{}))
	return stage
}

// open: the item as the core perceptors see it (read-write); not an item: skipped
type open struct{}

func (open) Decorate(in *flow.RawItem) (exif_core.RawItemRW, error) {
	if in.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	return in, nil
}
func (open) Stop() {}

// release: back to the item the next stages carry
type release struct{}

func (release) Decorate(in exif_core.RawItemRW) (*flow.RawItem, error) {
	it, ok := in.(*flow.RawItem)
	if !ok {
		return nil, fmt.Errorf("core perceptor returned %T, want *flow.RawItem", in)
	}
	return it, nil
}
func (release) Stop() {}
