package chain

import "context"

// Source: the logic of a chain's entry — one pass: every value it finds, emitted
// (emit is false once the chain stops). Optional: Stopper.
type Source[T any] interface {
	Start(ctx context.Context, emit func(T) bool) error
}

type entry[T any] struct {
	out   *Pipe[T]
	logic Source[T]
}

// Entry: the chain's one input, an output only; the chain starts it on every pass
// (Run), and after its values it flushes
func Entry[T any](out *Pipe[T], logic Source[T]) Processor {
	out.writers++
	return &entry[T]{out, logic}
}

func (e *entry[T]) run(r runtime) {
	defer stop(e.logic)
	emit := func(v T) bool { return e.out.send(r.ctx, msg[T]{v: v}) }
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-r.passes:
			r.report(e.logic.Start(r.ctx, emit))
			e.out.send(r.ctx, msg[T]{flush: true})
		}
	}
}
