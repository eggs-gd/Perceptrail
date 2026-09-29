package chain

import (
	"context"
)

// Expander turns one input into any number of outputs (none, one, many), in order.
// E.g. a grouper that buffers files and emits every group of a directory at once.
type Expander[Ti any, To any] interface {
	worker
	Expand(Ti) ([]To, error)
}

type expanderRunner[Ti any, To any] struct {
	cherr     chan<- error
	chin      <-chan Ti
	chout     chan<- To
	processor Expander[Ti, To]
}

func (e *expanderRunner[Ti, To]) setErrorChannel(cherr chan<- error) {
	e.cherr = cherr
}

func (e *expanderRunner[Ti, To]) Process(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			e.processor.Stop()
			return

		case input, ok := <-e.chin:
			if !ok {
				e.processor.Stop()
				return
			}
			res, err := e.processor.Expand(input)
			if err != nil {
				e.cherr <- err
				continue
			}
			for _, o := range res {
				select {
				case e.chout <- o:
				case <-ctx.Done():
					e.processor.Stop()
					return
				}
			}
		}
	}
}

func NewExpander[Ti any, To any](chin <-chan Ti, chout chan<- To, processor Expander[Ti, To]) Processor {
	return &expanderRunner[Ti, To]{
		chin:      chin,
		chout:     chout,
		processor: processor,
	}
}
