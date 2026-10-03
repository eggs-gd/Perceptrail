// Package plugins: the fourth stage of the import — the external perceptors (Go
// plugins) over the item the core has perceived. They only read it (api.RawItemR):
// what they find goes into their storages, the commit writes it.
//
//	each external perceptor, a step each, in the plugin manager's order
package plugins

import (
	"fmt"

	"perceptrail/gontroller/pkg/importer/identify"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// New: perceptors — the external perceptors, in order; in — the items the core has
// perceived; out — the same, perceived by the external ones (none: as they are). Its
// errors go to the chain it runs in.
func New(perceptors []api.ExifPerceptor, in, out *chain.Pipe[*identify.Item], logger *l.Logger) chain.Processor {
	var steps []chain.Decorator[*identify.Item, *identify.Item]
	for _, p := range perceptors {
		d := p.Decorator(logger.Named(p.Name()))
		if d == nil {
			logger.Error("EXIF plugin has no decorator, skipped", l.String("plugin", p.Name()))
			continue
		}
		steps = append(steps, perceive{name: p.Name(), logic: d})
	}
	return chain.Series(in, out, steps...)
}

// The external perceptors only read the item
var _ api.RawItemR = (*identify.Item)(nil)

// perceive: an external perceptor's logic over the item the stages carry; it sees
// the item read-only
type perceive struct {
	name  string
	logic chain.Decorator[api.RawItemR, api.RawItemR]
}

func (p perceive) Decorate(in *identify.Item) (*identify.Item, error) {
	out, err := p.logic.Decorate(in)
	if err != nil {
		return nil, err
	}
	it, ok := out.(*identify.Item)
	if !ok {
		return nil, fmt.Errorf("EXIF plugin %s returned %T, want the item it got", p.name, out)
	}
	return it, nil
}

func (p perceive) Stop() {
	if s, ok := p.logic.(chain.Stopper); ok {
		s.Stop()
	}
}
