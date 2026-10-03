package chain

// Router: a step's logic that picks one output for a value (its index). Optional:
// Stopper.
type Router[T any] interface {
	Route(T) (int, error)
}

type route[T any] struct {
	in    *Pipe[T]
	outs  []*Pipe[T]
	logic Router[T]
}

// Route: a value to one output, a flush to every output
func Route[T any](in *Pipe[T], outs []*Pipe[T], logic Router[T]) Processor {
	for _, o := range outs {
		o.writers++
	}
	return &route[T]{in, outs, logic}
}

func (s *route[T]) run(r runtime) {
	defer stop(s.logic)
	s.in.receive(r.ctx, func(v T) {
		i, err := s.logic.Route(v)
		if err != nil {
			r.report(err)
		} else if i >= 0 && i < len(s.outs) {
			s.outs[i].send(r.ctx, msg[T]{v: v})
		}
	}, func() {
		for _, o := range s.outs {
			o.send(r.ctx, msg[T]{flush: true})
		}
	})
}
