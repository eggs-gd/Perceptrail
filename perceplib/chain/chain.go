package chain

import (
	"context"
	"errors"
	"sync"
)

// ErrSkippedItem: a step drops the value on purpose (buffered, unchanged, not
// wanted) — not an error: it never reaches the error channel
var ErrSkippedItem = errors.New("skipped item")

// Processor: a step or a chain of steps, built by this package's constructors
type Processor interface {
	run(ctx context.Context, errch chan<- error)
}

// Stopper: a step's logic may have one; it is called once, when the step ends
type Stopper interface {
	Stop()
}

// Flusher: a step's logic may hold values (a group not complete yet); on a flush it
// gives them, and they go out before the flush
type Flusher[To any] interface {
	Flush() ([]To, error)
}

// Chain: steps that run together until the context ends
type Chain struct {
	errch chan<- error
	steps []Processor
}

// New: errch gets the steps' errors; nil: the errors of the chain this one runs in
func New(errch chan<- error) *Chain {
	return &Chain{errch: errch}
}

func (c *Chain) AddStep(p Processor) {
	c.steps = append(c.steps, p)
}

// Process runs every step until ctx ends
func (c *Chain) Process(ctx context.Context) {
	c.run(ctx, nil)
}

func (c *Chain) run(ctx context.Context, parent chan<- error) {
	errch := c.errch
	if errch == nil {
		errch = parent
	}
	var wg sync.WaitGroup
	for _, s := range c.steps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.run(ctx, errch)
		}()
	}
	wg.Wait()
}

// report: an error to errch (a skip is dropped; no channel: dropped)
func report(ctx context.Context, errch chan<- error, err error) {
	if err == nil || errors.Is(err, ErrSkippedItem) || errch == nil {
		return
	}
	select {
	case errch <- err:
	case <-ctx.Done():
	}
}

func stop(logic any) {
	if s, ok := logic.(Stopper); ok {
		s.Stop()
	}
}

// flushOut: what a Flusher holds, then the flush
func flushOut[To any](ctx context.Context, errch chan<- error, logic any, out *Pipe[To], b *batch) {
	if f, ok := logic.(Flusher[To]); ok {
		held, err := f.Flush()
		report(ctx, errch, err)
		for _, v := range held {
			out.send(ctx, msg[To]{v: v})
		}
	}
	out.forward(ctx, b)
}
