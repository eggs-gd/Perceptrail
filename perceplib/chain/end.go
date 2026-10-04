package chain

// Consumer: the logic of a chain's end — every value consumed. Optional: Stopper.
type Consumer[T any] interface {
	Consume(T) error
}

type end[T any] struct {
	in    <-chan T
	logic Consumer[T]
}

// End: a chain's end
func End[T any](in <-chan T, logic Consumer[T]) Processor {
	return &end[T]{in, logic}
}

func (*end[T]) outputs() []output { return nil }

func (s *end[T]) run(r runtime) {
	defer stop(s.logic)
	receive(r.ctx, s.in, func(v T) { r.report(s.logic.Consume(v)) })
}
