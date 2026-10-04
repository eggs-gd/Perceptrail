package chain

// Decorator: a step's logic — one value in, one out (or an error). Optional: Stopper,
// Flusher.
type Decorator[Ti, To any] interface {
	Decorate(Ti) (To, error)
}

type decoratorRunner[Ti, To any] struct {
	in    <-chan Ti
	out   chan<- To
	logic Decorator[Ti, To]
}

func NewDecorator[Ti, To any](in <-chan Ti, out chan<- To, logic Decorator[Ti, To]) Processor {
	return &decoratorRunner[Ti, To]{in, out, logic}
}

func (s *decoratorRunner[Ti, To]) outputs() []output { return []output{outputOf(s.out)} }

func (s *decoratorRunner[Ti, To]) run(r runtime) {
	defer stop(s.logic)
	ended := receive(r.ctx, s.in, func(v Ti) {
		if o, err := s.logic.Decorate(v); err != nil {
			r.report(err)
		} else {
			send(r.ctx, s.out, o)
		}
	})
	if ended {
		flushOut(r, s.logic, s.out)
	}
}
