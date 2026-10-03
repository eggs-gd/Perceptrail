// Package chain: steps that run concurrently, connected by typed pipes. A pipe
// carries values and a flush: a batch is complete (a walk). Every step passes the
// flush on after the values before it; where several steps write one pipe, it
// passes once every one of them has flushed. When the flush has reached every end
// of the chain, the chain says so (Done).
package chain

import (
	"context"
	"errors"
	"sync"
)

// ErrSkippedItem: a step drops the value on purpose — not an error, never reported
var ErrSkippedItem = errors.New("skipped item")

// Processor: a step, or a chain of steps
type Processor interface {
	run(r runtime)
}

// runtime: what a running step gets from its chain
type runtime struct {
	ctx   context.Context
	errch chan<- error
	ended func() // an end got the flush
}

// Stopper: a step's logic may have one; it is called once, when the step ends
type Stopper interface{ Stop() }

// Flusher: a step's logic may hold values; on a flush they go out first
type Flusher[To any] interface {
	Flush() ([]To, error)
}

// Chain: steps that run together until the context ends
type Chain struct {
	errch chan<- error
	steps []Processor
	ends  int
	done  chan struct{}
}

// New: errch gets the steps' errors (nil: the errors of the chain this one runs in)
func New(errch chan<- error) *Chain {
	return &Chain{errch: errch, done: make(chan struct{}, 1)}
}

func (c *Chain) AddStep(p Processor) {
	c.steps = append(c.steps, p)
	c.ends += endsOf(p)
}

// Done: a flush has reached every end of the chain (its batch went through)
func (c *Chain) Done() <-chan struct{} { return c.done }

// Process runs every step until ctx ends
func (c *Chain) Process(ctx context.Context) {
	c.run(runtime{ctx: ctx})
}

func (c *Chain) run(r runtime) {
	if c.errch != nil {
		r.errch = c.errch
	}
	if r.ended == nil { // the top chain counts its ends
		var mu sync.Mutex
		ended := 0
		r.ended = func() {
			mu.Lock()
			defer mu.Unlock()
			if ended++; ended == c.ends {
				ended = 0
				select {
				case c.done <- struct{}{}:
				default:
				}
			}
		}
	}
	var wg sync.WaitGroup
	for _, s := range c.steps {
		wg.Go(func() { s.run(r) })
	}
	wg.Wait()
}

func endsOf(p Processor) int {
	switch s := p.(type) {
	case *Chain:
		return s.ends
	case ender:
		return 1
	}
	return 0
}

type ender interface{ isEnd() }

func (r runtime) report(err error) {
	if err == nil || errors.Is(err, ErrSkippedItem) || r.errch == nil {
		return
	}
	select {
	case r.errch <- err:
	case <-r.ctx.Done():
	}
}

func stop(logic any) {
	if s, ok := logic.(Stopper); ok {
		s.Stop()
	}
}
