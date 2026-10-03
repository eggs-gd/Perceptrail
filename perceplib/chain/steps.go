package chain

import (
	"context"
	"sync"
)

// Decorator: a step's logic — one value in, one out (or an error; ErrSkippedItem:
// dropped). Optional: Stopper, Flusher[To].
type Decorator[Ti any, To any] interface {
	Decorate(Ti) (To, error)
}

type decorateStep[Ti, To any] struct {
	in    *Pipe[Ti]
	out   *Pipe[To]
	logic Decorator[Ti, To]
}

// Decorate: logic between in and out
func Decorate[Ti, To any](in *Pipe[Ti], out *Pipe[To], logic Decorator[Ti, To]) Processor {
	out.writers++
	return &decorateStep[Ti, To]{in: in, out: out, logic: logic}
}

func (s *decorateStep[Ti, To]) run(ctx context.Context, errch chan<- error) {
	defer stop(s.logic)
	s.in.receive(ctx, func(v Ti) {
		r, err := s.logic.Decorate(v)
		if err != nil {
			report(ctx, errch, err)
			return
		}
		s.out.send(ctx, msg[To]{v: r})
	}, func() {
		flushOut(ctx, errch, s.logic, s.out)
	})
}

type parallelStep[Ti, To any] struct {
	n     int
	in    *Pipe[Ti]
	out   *Pipe[To]
	logic Decorator[Ti, To]
}

// Parallel: the same logic on n workers (it must be safe for concurrent use); the
// values may come out in another order. A flush waits until every value before it is
// done.
func Parallel[Ti, To any](n int, in *Pipe[Ti], out *Pipe[To], logic Decorator[Ti, To]) Processor {
	out.writers++
	return &parallelStep[Ti, To]{n: max(n, 1), in: in, out: out, logic: logic}
}

func (s *parallelStep[Ti, To]) run(ctx context.Context, errch chan<- error) {
	defer stop(s.logic)
	work := make(chan Ti)
	var workers, inFlight sync.WaitGroup
	for range s.n {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for v := range work {
				r, err := s.logic.Decorate(v)
				if err != nil {
					report(ctx, errch, err)
				} else {
					s.out.send(ctx, msg[To]{v: r})
				}
				inFlight.Done()
			}
		}()
	}
	s.in.receive(ctx, func(v Ti) {
		inFlight.Add(1)
		select {
		case work <- v:
		case <-ctx.Done():
			inFlight.Done()
		}
	}, func() {
		inFlight.Wait()
		flushOut(ctx, errch, s.logic, s.out)
	})
	close(work)
	workers.Wait()
}

// Router: a step's logic that sends a value to one of its outputs (an index;
// ErrSkippedItem: none). A flush goes to every output. Optional: Stopper.
type Router[T any] interface {
	Route(T) (int, error)
}

type routeStep[T any] struct {
	in    *Pipe[T]
	outs  []*Pipe[T]
	logic Router[T]
}

// Route: logic picks the output of every value of in
func Route[T any](in *Pipe[T], outs []*Pipe[T], logic Router[T]) Processor {
	for _, o := range outs {
		o.writers++
	}
	return &routeStep[T]{in: in, outs: outs, logic: logic}
}

func (s *routeStep[T]) run(ctx context.Context, errch chan<- error) {
	defer stop(s.logic)
	s.in.receive(ctx, func(v T) {
		i, err := s.logic.Route(v)
		if err != nil {
			report(ctx, errch, err)
			return
		}
		if i >= 0 && i < len(s.outs) {
			s.outs[i].send(ctx, msg[T]{v: v})
		}
	}, func() {
		for _, o := range s.outs {
			o.send(ctx, msg[T]{flush: true})
		}
	})
}

// Emitter: what a Source writes to — values, and a flush when a batch is complete.
// Both return false once the chain stops.
type Emitter[T any] interface {
	Emit(T) bool
	Flush() bool
}

// Source: the logic of a chain's entry point — it runs until ctx ends. Optional:
// Stopper.
type Source[T any] interface {
	Run(ctx context.Context, out Emitter[T])
}

type emitter[T any] struct {
	ctx context.Context
	out *Pipe[T]
}

func (e emitter[T]) Emit(v T) bool { return e.out.send(e.ctx, msg[T]{v: v}) }
func (e emitter[T]) Flush() bool   { return e.out.send(e.ctx, msg[T]{flush: true}) }

type entryStep[T any] struct {
	out   *Pipe[T]
	logic Source[T]
}

// Entry: logic feeds out
func Entry[T any](out *Pipe[T], logic Source[T]) Processor {
	out.writers++
	return &entryStep[T]{out: out, logic: logic}
}

func (s *entryStep[T]) run(ctx context.Context, _ chan<- error) {
	defer stop(s.logic)
	s.logic.Run(ctx, emitter[T]{ctx: ctx, out: s.out})
}

type sinkStep[T any] struct {
	in      *Pipe[T]
	each    func(T)
	flushed func()
}

// Sink: the end of a chain — each value to each, and flushed when a flush arrives
// (every value before it went through the whole chain)
func Sink[T any](in *Pipe[T], each func(T), flushed func()) Processor {
	return &sinkStep[T]{in: in, each: each, flushed: flushed}
}

func (s *sinkStep[T]) run(ctx context.Context, _ chan<- error) {
	s.in.receive(ctx, func(v T) {
		if s.each != nil {
			s.each(v)
		}
	}, func() {
		if s.flushed != nil {
			s.flushed()
		}
	})
}

type pass[T any] struct{}

func (pass[T]) Decorate(v T) (T, error) { return v, nil }

// Series: logics one after another from in to out, a step each (none: values pass
// as they are)
func Series[T any](in, out *Pipe[T], logics ...Decorator[T, T]) Processor {
	c := New(nil)
	if len(logics) == 0 {
		c.AddStep(Decorate[T, T](in, out, pass[T]{}))
		return c
	}
	prev := in
	for i, logic := range logics {
		next := out
		if i < len(logics)-1 {
			next = NewPipe[T](0)
		}
		c.AddStep(Decorate(prev, next, logic))
		prev = next
	}
	return c
}
