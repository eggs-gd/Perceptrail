package chain

import "context"

// Source: the logic of a chain's entry — one pass: every value it finds, emitted
// (emit is false once the chain stops). Optional: Stopper.
type Source[T any] interface {
	Start(ctx context.Context, emit func(T) bool) error
}

type entry[T any] struct {
	out   chan<- T
	logic Source[T]
}

// Entry: the chain's one input, an output only; when it is done, its output closes
func Entry[T any](out chan<- T, logic Source[T]) Processor {
	return &entry[T]{out, logic}
}

func (e *entry[T]) outputs() []output { return []output{outputOf(e.out)} }

func (e *entry[T]) run(r runtime) {
	defer stop(e.logic)
	r.report(e.logic.Start(r.ctx, func(v T) bool { return send(r.ctx, e.out, v) }))
}
