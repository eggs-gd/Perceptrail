package chain

// Decorator: a step's logic — one value in, one out (or an error). Optional: Stopper,
// Flusher.
type Decorator[Ti, To any] interface {
	Decorate(Ti) (To, error)
}

type decorate[Ti, To any] struct {
	in    *Pipe[Ti]
	out   *Pipe[To]
	logic Decorator[Ti, To]
}

func Decorate[Ti, To any](in *Pipe[Ti], out *Pipe[To], logic Decorator[Ti, To]) Processor {
	out.writers++
	return &decorate[Ti, To]{in, out, logic}
}

func (s *decorate[Ti, To]) run(r runtime) {
	defer stop(s.logic)
	s.in.receive(r.ctx, func(v Ti) {
		if o, err := s.logic.Decorate(v); err != nil {
			r.report(err)
		} else {
			s.out.send(r.ctx, msg[To]{v: o})
		}
	}, func() { flushOut(r, s.logic, s.out) })
}
