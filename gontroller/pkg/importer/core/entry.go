// Package core: the third stage of the import — the core perceptors (built in: the
// date, the size, …) over the identified item. They write into it
// (exif_core.RawItemRW): what they find goes into the item and their storages.
//
//	each core perceptor, a step each, in the plugin manager's order
package core

import (
	"fmt"

	"perceptrail/gontroller/pkg/importer/identify"
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// New: perceptors — the core perceptors, in order; in — the identified items; out
// — the same, perceived by the core (none: as they are). Its errors go to the chain
// it runs in.
func New(perceptors []exif_core.ExifCorePerceptor, in, out *chain.Pipe[*identify.Item], logger *l.Logger) chain.Processor {
	var steps []chain.Decorator[*identify.Item, *identify.Item]
	for _, p := range perceptors {
		d := p.Decorator(logger.Named(p.Name()))
		if d == nil {
			logger.Error("Core perceptor has no decorator, skipped", l.String("plugin", p.Name()))
			continue
		}
		steps = append(steps, perceive{name: p.Name(), logic: d})
	}
	return chain.Series(in, out, steps...)
}

// The core perceptors read and write the item
var _ exif_core.RawItemRW = (*identify.Item)(nil)

// perceive: a core perceptor's logic over the item the stages carry
type perceive struct {
	name  string
	logic chain.Decorator[exif_core.RawItemRW, exif_core.RawItemRW]
}

func (p perceive) Decorate(in *identify.Item) (*identify.Item, error) {
	out, err := p.logic.Decorate(in)
	if err != nil {
		return nil, err
	}
	it, ok := out.(*identify.Item)
	if !ok {
		return nil, fmt.Errorf("core perceptor %s returned %T, want the item it got", p.name, out)
	}
	return it, nil
}

func (p perceive) Stop() {
	if s, ok := p.logic.(chain.Stopper); ok {
		s.Stop()
	}
}
