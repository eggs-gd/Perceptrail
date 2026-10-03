// Package chain: steps that run concurrently, connected by typed pipes. A pipe
// carries values and the flush signal: a source flushes when a batch is complete (a
// walk), every step passes it on after the values before it, so a flush at the end of
// the chain means the batch is done.
package chain

import "context"

// msg: a value, or the flush signal
type msg[T any] struct {
	v     T
	flush bool
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

func (p *Pipe[T]) send(ctx context.Context, m msg[T]) bool {
	select {
	case p.ch <- m:
		return true
	case <-ctx.Done():
		return false
	}
}

// receive reads p until ctx ends: every value to each, a flush to flush once every
// writer has flushed
func (p *Pipe[T]) receive(ctx context.Context, each func(T), flush func()) {
	flushed := 0
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-p.ch:
			if !m.flush {
				each(m.v)
				continue
			}
			if flushed++; flushed >= p.writers {
				flushed = 0
				flush()
			}
		}
	}
}
