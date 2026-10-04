package chain

// Decorator: a step's logic — one value in, one out (or an error). Optional: Stopper,
// Flusher.
type Decorator[Ti, To any] interface {
	Decorate(Ti) (To, error)
}

type decorate[Ti, To any] struct {
	in    <-chan Ti
	out   chan<- To
	logic Decorator[Ti, To]
}

func Decorate[Ti, To any](in <-chan Ti, out chan<- To, logic Decorator[Ti, To]) Processor {
	return &decorate[Ti, To]{in, out, logic}
}

func (s *decorate[Ti, To]) outputs() []output { return []output{outputOf(s.out)} }

func (s *decorate[Ti, To]) run(r runtime) {
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
