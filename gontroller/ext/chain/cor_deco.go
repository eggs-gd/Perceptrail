package chain

import "context"

type Decorator[Ti any, To any] interface {
	Decorate(in Ti) (To, error)
	Close()
}

type decoratorRunner[Ti any, To any] struct {
	ctx    context.Context
	cancel context.CancelFunc

	cherr     chan<- error
	chin      <-chan Ti
	chout     chan<- To
	processor Decorator[Ti, To]
}

func (d *decoratorRunner[Ti, To]) setErrorChannel(cherr chan<- error) {
	d.cherr = cherr
}

func (d *decoratorRunner[Ti, To]) Process(ctx context.Context) {
	d.ctx, d.cancel = context.WithCancel(ctx)
	for input := range d.chin {
		res, err := d.processor.Decorate(input)
		if err != nil {
			d.cherr <- err
		} else {
			d.chout <- res
		}
	}
}

func (d *decoratorRunner[Ti, To]) Close() {
	d.processor.Close()
}

func NewDecorator[Ti any, To any](chin <-chan Ti, chout chan<- To, processor Decorator[Ti, To]) Processor {
	return &decoratorRunner[Ti, To]{
		chin:      chin,
		chout:     chout,
		processor: processor,
	}
}
