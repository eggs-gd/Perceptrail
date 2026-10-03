package chain

import "sync"

type parallel[Ti, To any] struct {
	n     int
	in    *Pipe[Ti]
	out   *Pipe[To]
	logic Decorator[Ti, To]
}

// Parallel: the same logic on n workers (safe for concurrent use; the order may
// change); a flush waits for every value before it
func Parallel[Ti, To any](n int, in *Pipe[Ti], out *Pipe[To], logic Decorator[Ti, To]) Processor {
	out.writers++
	return &parallel[Ti, To]{max(n, 1), in, out, logic}
}

func (s *parallel[Ti, To]) run(r runtime) {
	defer stop(s.logic)
	work := make(chan Ti)
	var workers, inFlight sync.WaitGroup
	for range s.n {
		workers.Go(func() {
			for v := range work {
				if o, err := s.logic.Decorate(v); err != nil {
					r.report(err)
				} else {
					s.out.send(r.ctx, msg[To]{v: o})
				}
				inFlight.Done()
			}
		})
	}
	s.in.receive(r.ctx, func(v Ti) {
		inFlight.Add(1)
		select {
		case work <- v:
		case <-r.ctx.Done():
			inFlight.Done()
		}
	}, func() {
		inFlight.Wait()
		flushOut(r, s.logic, s.out)
	})
	close(work)
	workers.Wait()
}
