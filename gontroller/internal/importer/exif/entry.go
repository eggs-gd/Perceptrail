// Package exif: the import's EXIF perceptors over the identified item — the
// built-in ones (they write into it: date, size, length), the external Go plugins
// (they only read it), then their values kept, a row in each one's storage. At the
// service's start their rows are reconciled with the items (Reconcile). It reads
// the plugin registry itself.
//
//	each built-in perceptor → each external one → keep
package exif

import (
	"fmt"

	"perceptrail/gontroller/internal/importer/identify"
	"perceptrail/gontroller/internal/perceptor/builtin"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// The item as the perceptors see it: the built-in ones read and write it, the
// external ones only read it
var (
	_ builtin.Item = (*identify.Item)(nil)
	_ api.RawItemR = (*identify.Item)(nil)
)

// New: in — the identified items; out — the same, perceived, their values kept.
// Its errors go to the chain it runs in.
func New(logger *l.Logger, in <-chan *identify.Item, out chan<- *identify.Item) chain.Processor {
	var steps []chain.Decorator[*identify.Item, *identify.Item]
	for _, p := range corePerceptors() {
		if d := p.Decorator(logger.Named(p.Name())); d != nil {
			steps = append(steps, perceive[builtin.Item]{name: p.Name(), logic: d})
		} else {
			logger.Error("Core perceptor has no decorator, skipped", l.String("plugin", p.Name()))
		}
	}
	for _, p := range externalPerceptors() {
		if d := p.Decorator(logger.Named(p.Name())); d != nil {
			steps = append(steps, perceive[api.RawItemR]{name: p.Name(), logic: d})
		} else {
			logger.Error("EXIF plugin has no decorator, skipped", l.String("plugin", p.Name()))
		}
	}
	// One after another, a step each
	c := chain.NewChainProcessor(nil)
	from := in
	for _, d := range steps {
		to := make(chan *identify.Item)
		c.AddStep(chain.NewDecorator(from, to, d))
		from = to
	}
	c.AddStep(chain.NewDecorator(from, out, keep{}))
	return c
}

// perceive: a perceptor's logic over the item the steps carry, seen as T (read-write
// for a built-in one, read-only for a plugin)
type perceive[T any] struct {
	name  string
	logic chain.Decorator[T, T]
}

func (p perceive[T]) Decorate(in *identify.Item) (*identify.Item, error) {
	view, _ := any(in).(T)
	out, err := p.logic.Decorate(view)
	if err != nil {
		return nil, err
	}
	it, ok := any(out).(*identify.Item)
	if !ok {
		return nil, fmt.Errorf("perceptor %s returned %T, want the item it got", p.name, out)
	}
	return it, nil
}

func (p perceive[T]) Stop() {
	if s, ok := p.logic.(chain.Stopper); ok {
		s.Stop()
	}
}

// keep: the values the perceptors put on the item, a row in each one's storage (its
// value, or "processed, nothing found"). Before the item is published: an item
// published without them would be taken as done; a crash between the two leaves an
// item that is not, the next walk sends it again.
type keep struct{}

func (keep) Decorate(it *identify.Item) (*identify.Item, error) {
	if it.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	if err := saveValues(it.Item.Guid, it.StoreValues); err != nil {
		return nil, err
	}
	return it, nil
}
