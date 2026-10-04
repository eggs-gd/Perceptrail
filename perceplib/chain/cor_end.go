package chain

// Consumer: the logic of a chain's end — every value consumed. Optional: Stopper.
type Consumer[T any] interface {
	Consume(T) error
}

type endRunner[T any] struct {
	in    <-chan T
	logic Consumer[T]
}

// NewEnd: a chain's end
func NewEnd[T any](in <-chan T, logic Consumer[T]) Processor {
	return &endRunner[T]{in, logic}
}

func (*endRunner[T]) outputs() []output { return nil }

func (s *endRunner[T]) run(r runtime) {
	defer stop(s.logic)
	receive(r.ctx, s.in, func(v T) { r.report(s.logic.Consume(v)) })
}
