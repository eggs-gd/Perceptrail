package chain

type IDecorator[Ti any, To any] interface {
	Decorate(in Ti) To
}

type decoratorRunner[Ti any, To any] struct {
	chin      <-chan Ti
	chout     chan<- To
	processor IDecorator[Ti, To]
}

func (d *decoratorRunner[Ti, To]) Process() {
	for input := range d.chin {
		result := d.processor.Decorate(input)
		d.chout <- result
	}
}

func NewDecorator[Ti any, To any](chin <-chan Ti, chout chan<- To, decorator IDecorator[Ti, To]) ControlBase {
	return &decoratorRunner[Ti, To]{
		chin, chout, decorator,
	}
}
