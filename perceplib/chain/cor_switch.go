package chain

// Switcher: a step's logic that picks one output for a value (its index). Optional:
// Stopper.
type Switcher[T any] interface {
	Switch(T) (int, error)
}

type switchRunner[T any] struct {
	in    <-chan T
	outs  []chan<- T
	logic Switcher[T]
}

// NewSwitch: a value to one output; when the input ends, every output is done
func NewSwitch[T any](in <-chan T, outs []chan<- T, logic Switcher[T]) Processor {
	return &switchRunner[T]{in, outs, logic}
}

func (s *switchRunner[T]) outputs() []output {
	out := make([]output, len(s.outs))
	for i, o := range s.outs {
		out[i] = outputOf(o)
	}
	return out
}

func (s *switchRunner[T]) run(r runtime) {
	defer stop(s.logic)
	receive(r.ctx, s.in, func(v T) {
		i, err := s.logic.Switch(v)
		if err != nil {
			r.report(err)
		} else if i >= 0 && i < len(s.outs) {
			send(r.ctx, s.outs[i], v)
		}
	})
}
