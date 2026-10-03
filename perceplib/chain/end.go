package chain

// Consumer: the logic of a chain's end — every value consumed. Optional: Stopper.
type Consumer[T any] interface {
	Consume(T) error
}

type end[T any] struct {
	in    *Pipe[T]
	logic Consumer[T]
}

// End: a chain's end; when a flush has reached every end, the chain is Done
func End[T any](in *Pipe[T], logic Consumer[T]) Processor {
	return &end[T]{in, logic}
}

func (*end[T]) isEnd() {}

func (s *end[T]) run(r runtime) {
	defer stop(s.logic)
	s.in.receive(r.ctx, func(v T) { r.report(s.logic.Consume(v)) }, r.ended)
}
