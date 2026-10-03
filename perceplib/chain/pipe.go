package chain

import "context"

// msg: a value, or the flush
type msg[T any] struct {
	v     T
	flush bool
}

// Pipe: a typed channel between steps. The steps that write to it register when
// built (its writers); a reader passes a flush on once every writer has flushed.
type Pipe[T any] struct {
	ch      chan msg[T]
	writers int
}

func NewPipe[T any](buffer int) *Pipe[T] {
	return &Pipe[T]{ch: make(chan msg[T], buffer)}
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
			} else if flushed++; flushed >= p.writers {
				flushed = 0
				flush()
			}
		}
	}
}

// flushOut: what a Flusher holds, then the flush
func flushOut[To any](r runtime, logic any, out *Pipe[To]) {
	if f, ok := logic.(Flusher[To]); ok {
		held, err := f.Flush()
		r.report(err)
		for _, v := range held {
			out.send(r.ctx, msg[To]{v: v})
		}
	}
	out.send(r.ctx, msg[To]{flush: true})
}
