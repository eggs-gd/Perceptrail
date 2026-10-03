// Package chain: steps that run concurrently, connected by typed pipes. A pipe
// carries values and the flush signal: a source flushes when a batch is complete (a
// walk), every step passes it on after the values before it, and the source hears
// when the flush has left every end of the chain — the batch is done.
package chain

import (
	"context"
	"sync"
	"sync/atomic"
)

// msg: a value, or the flush signal (flush != nil)
type msg[T any] struct {
	v     T
	flush *batch
}

// batch: one flush on its way through the chain. Its tokens are its copies in the
// pipes: a step that passes it on adds one per output before it consumes what it
// got (a Route multiplies it, a barrier joins it); a chain's end consumes it. The
// last token consumed: the batch is done.
type batch struct {
	tokens atomic.Int64
	done   chan struct{}
	once   sync.Once
}

func newBatch() *batch {
	b := &batch{done: make(chan struct{})}
	b.tokens.Store(1)
	return b
}

func (b *batch) add(n int) { b.tokens.Add(int64(n)) }

func (b *batch) consume() {
	if b.tokens.Add(-1) == 0 {
		b.once.Do(func() { close(b.done) })
	}
}

// wait: the batch is done; false if ctx ended first
func (b *batch) wait(ctx context.Context) bool {
	select {
	case <-b.done:
		return true
	case <-ctx.Done():
		return false
	}
}

// Pipe: a typed channel between steps. The steps that write to it register when
// they are built (its writers): a reader passes a flush on only once every writer
// has flushed (the barrier where branches join).
type Pipe[T any] struct {
	ch      chan msg[T]
	writers int
}

// NewPipe: buffer is the channel's (0: unbuffered)
func NewPipe[T any](buffer int) *Pipe[T] {
	return &Pipe[T]{ch: make(chan msg[T], buffer)}
}

// Send: a value from outside the chain (not a writer: it never flushes); false if ctx
// ended first
func (p *Pipe[T]) Send(ctx context.Context, v T) bool {
	return p.send(ctx, msg[T]{v: v})
}

// Flush: a flush from outside the chain, returning once it has left every end of the
// chain (a test driving a sub-chain value by value: Send, Flush — every value sent
// before went through); false if ctx ended first
func (p *Pipe[T]) Flush(ctx context.Context) bool {
	b := newBatch()
	return p.send(ctx, msg[T]{flush: b}) && b.wait(ctx)
}

// forward: a flush passed on (one more token of its batch)
func (p *Pipe[T]) forward(ctx context.Context, b *batch) {
	b.add(1)
	if !p.send(ctx, msg[T]{flush: b}) {
		b.consume() // nobody will: the chain stops
	}
}

func (p *Pipe[T]) send(ctx context.Context, m msg[T]) bool {
	select {
	case p.ch <- m:
		return true
	case <-ctx.Done():
		return false
	}
}

// receive reads p until ctx ends: every value to each, a flush to flush once every
// writer has flushed (with its batch: flush passes it on, or not at a chain's end);
// the tokens received are consumed after
func (p *Pipe[T]) receive(ctx context.Context, each func(T), flush func(*batch)) {
	var held []*batch
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-p.ch:
			if m.flush == nil {
				each(m.v)
				continue
			}
			if held = append(held, m.flush); len(held) >= p.writers {
				flush(held[0])
				for _, b := range held {
					b.consume()
				}
				held = nil
			}
		}
	}
}
