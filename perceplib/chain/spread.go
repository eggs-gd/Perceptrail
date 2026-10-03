package chain

import "context"

// Spreader: a step's logic that makes many values of one (a walk request: every file
// found); after each, the step flushes — a batch. Optional: Stopper.
type Spreader[Ti, To any] interface {
	Spread(ctx context.Context, v Ti, emit func(To) bool) error
}

type spread[Ti, To any] struct {
	in    *Pipe[Ti]
	out   *Pipe[To]
	logic Spreader[Ti, To]
}

func Spread[Ti, To any](in *Pipe[Ti], out *Pipe[To], logic Spreader[Ti, To]) Processor {
	out.writers++
	return &spread[Ti, To]{in, out, logic}
}

func (s *spread[Ti, To]) run(r runtime) {
	defer stop(s.logic)
	emit := func(v To) bool { return s.out.send(r.ctx, msg[To]{v: v}) }
	s.in.receive(r.ctx, func(v Ti) {
		r.report(s.logic.Spread(r.ctx, v, emit))
		s.out.send(r.ctx, msg[To]{flush: true})
	}, func() { s.out.send(r.ctx, msg[To]{flush: true}) })
}
