package chain

// Router: a step's logic that picks one output for a value (its index). Optional:
// Stopper.
type Router[T any] interface {
	Route(T) (int, error)
}

type route[T any] struct {
	in    <-chan T
	outs  []chan<- T
	logic Router[T]
}

// Route: a value to one output; when the input ends, every output is done
func Route[T any](in <-chan T, outs []chan<- T, logic Router[T]) Processor {
	return &route[T]{in, outs, logic}
}

func (s *route[T]) outputs() []output {
	out := make([]output, len(s.outs))
	for i, o := range s.outs {
		out[i] = outputOf(o)
	}
	return out
}

func (s *route[T]) run(r runtime) {
	defer stop(s.logic)
	receive(r.ctx, s.in, func(v T) {
		i, err := s.logic.Route(v)
		if err != nil {
			r.report(err)
		} else if i >= 0 && i < len(s.outs) {
			send(r.ctx, s.outs[i], v)
		}
	})
}
