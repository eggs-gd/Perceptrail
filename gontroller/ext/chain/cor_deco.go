package chain

type Decorator[Ti any, To any] interface {
	Decorate(in Ti) To
	Close()
}

type decoratorRunner[Ti any, To any] struct {
	chin      <-chan Ti
	chout     chan<- To
	processor Decorator[Ti, To]
}

func (d *decoratorRunner[Ti, To]) Process() {
	for input := range d.chin {
		result := d.processor.Decorate(input)
		d.chout <- result
	}
}

func (d *decoratorRunner[Ti, To]) Close() {
	d.processor.Close()
}

func NewDecorator[Ti any, To any](chin <-chan Ti, chout chan<- To, decorator Decorator[Ti, To]) Processor {
	return &decoratorRunner[Ti, To]{
		chin, chout, decorator,
	}
}
