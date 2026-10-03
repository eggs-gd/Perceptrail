// Package plugins: the fourth stage of the import — the external perceptors (Go
// plugins) over the item the core has perceived. They only read it (api.RawItemR):
// what they find goes into their storages, the commit writes it.
//
//	each external perceptor, in the plugin manager's order: to read-only → the
//	perceptor → back
package plugins

import (
	"fmt"

	"perceptrail/gontroller/pkg/importer/identify"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// New: perceptors — the external perceptors, in order; in — the items the core has
// perceived; out — the same, perceived by the external ones (none: passed on as they
// are). Its steps report to errch.
func New(perceptors []api.ExifPerceptor, in <-chan *identify.Item, out chan<- *identify.Item, errch chan error, logger *l.Logger) chain.ChainProcessor {
	stage := chain.NewChainProcessor(errch)

	// A perceptor's step reads its own channel and writes its own; the stage wires
	// them one after another
	type step struct {
		in, out chan api.RawItemR
		proc    chain.Processor
	}
	var steps []step
	for _, p := range perceptors {
		s := step{in: make(chan api.RawItemR, 1), out: make(chan api.RawItemR, 1)}
		if s.proc = p.NewProcessor(s.in, s.out, logger.Named(p.Name())); s.proc == nil {
			logger.Error("EXIF plugin returned no processor, skipped", l.String("plugin", p.Name()))
			continue
		}
		steps = append(steps, s)
	}
	if len(steps) == 0 {
		stage.AddStep(chain.NewDecorator(in, out, pass{}))
		return stage
	}

	prev := in
	for i, s := range steps {
		stage.AddStep(chain.NewDecorator(prev, s.in, toReadOnly{}))
		stage.AddStep(s.proc)
		if i == len(steps)-1 { // the last one writes out
			stage.AddStep(chain.NewDecorator(s.out, out, back{}))
			break
		}
		next := make(chan *identify.Item, 1)
		stage.AddStep(chain.NewDecorator(s.out, next, back{}))
		prev = next
	}
	return stage
}

// toReadOnly: what an external perceptor sees
type toReadOnly struct{}

func (toReadOnly) Decorate(in *identify.Item) (api.RawItemR, error) { return in, nil }
func (toReadOnly) Stop()                                            {}

// back: the item an external perceptor returned
type back struct{}

func (back) Decorate(in api.RawItemR) (*identify.Item, error) {
	it, ok := in.(*identify.Item)
	if !ok {
		return nil, fmt.Errorf("external EXIF plugin returned %T, want the item it got", in)
	}
	return it, nil
}
func (back) Stop() {}

type pass struct{}

func (pass) Decorate(in *identify.Item) (*identify.Item, error) { return in, nil }
func (pass) Stop()                                              {}
