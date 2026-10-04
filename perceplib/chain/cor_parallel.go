package chain

import "sync"

type parallelRunner[Ti, To any] struct {
	n     int
	in    <-chan Ti
	out   chan<- To
	logic Decorator[Ti, To]
}

// NewParallel: the same logic on n workers (safe for concurrent use; the order may
// change); when the input ends, after the last of them, what the logic holds
func NewParallel[Ti, To any](n int, in <-chan Ti, out chan<- To, logic Decorator[Ti, To]) Processor {
	return &parallelRunner[Ti, To]{max(n, 1), in, out, logic}
}

func (s *parallelRunner[Ti, To]) outputs() []output { return []output{outputOf(s.out)} }

func (s *parallelRunner[Ti, To]) run(r runtime) {
	defer stop(s.logic)
	var workers sync.WaitGroup
	for range s.n {
		workers.Go(func() {
			receive(r.ctx, s.in, func(v Ti) {
				if o, err := s.logic.Decorate(v); err != nil {
					r.report(err)
				} else {
					send(r.ctx, s.out, o)
				}
			})
		})
	}
	workers.Wait()
	if r.ctx.Err() == nil { // the input ended (not the pass)
		flushOut(r, s.logic, s.out)
	}
}
