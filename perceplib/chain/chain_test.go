package chain

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// collect: the values that reach out, and whether a flush came after them
type collect[T any] struct {
	mu      sync.Mutex
	got     []T
	flushes []int // len(got) at each flush
	done    chan struct{}
}

func newCollect[T any]() *collect[T] { return &collect[T]{done: make(chan struct{}, 10)} }

func (c *collect[T]) sink(in *Pipe[T]) Processor {
	return Sink(in, func(v T) {
		c.mu.Lock()
		c.got = append(c.got, v)
		c.mu.Unlock()
	}, func() {
		c.mu.Lock()
		c.flushes = append(c.flushes, len(c.got))
		c.mu.Unlock()
		c.done <- struct{}{}
	})
}

func (c *collect[T]) wait(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("no flush reached the end")
	}
}

// source: emits vs, then flushes
type source[T any] struct{ vs []T }

func (s source[T]) Run(ctx context.Context, out Emitter[T]) {
	for _, v := range s.vs {
		out.Emit(v)
	}
	out.Flush()
	<-ctx.Done()
}

type fn[Ti, To any] func(Ti) (To, error)

func (f fn[Ti, To]) Decorate(v Ti) (To, error) { return f(v) }

func run(t *testing.T, c *Chain) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	go c.Process(ctx)
	return cancel
}

// Every value comes out before the flush that followed it
func TestFlushAfterValues(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	c := New(nil)
	c.AddStep(Entry(in, source[int]{[]int{1, 2, 3}}))
	c.AddStep(Decorate(in, out, fn[int, int](func(v int) (int, error) { return v * 10, nil })))
	c.AddStep(got.sink(out))
	defer run(t, c)()
	got.wait(t)
	if !slices.Equal(got.got, []int{10, 20, 30}) || got.flushes[0] != 3 {
		t.Errorf("got %v, flushes %v", got.got, got.flushes)
	}
}

// holder keeps every value until the flush
type holder struct{ held []int }

func (h *holder) Decorate(v int) (int, error) { h.held = append(h.held, v); return 0, ErrSkippedItem }
func (h *holder) Flush() ([]int, error)       { out := h.held; h.held = nil; return out, nil }

// A Flusher's values go out on the flush, before it
func TestFlusherBeforeFlush(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	c := New(nil)
	c.AddStep(Entry(in, source[int]{[]int{1, 2}}))
	c.AddStep(Decorate[int, int](in, out, &holder{}))
	c.AddStep(got.sink(out))
	defer run(t, c)()
	got.wait(t)
	if !slices.Equal(got.got, []int{1, 2}) || got.flushes[0] != 2 {
		t.Errorf("got %v, flushes %v", got.got, got.flushes)
	}
}

type parity struct{}

func (parity) Route(v int) (int, error) { return v % 2, nil }

// Route: a value to one output, the flush to every one; where branches join, the
// flush passes once every branch has flushed
func TestRouteAndBarrier(t *testing.T) {
	in, even, odd, joined := NewPipe[int](0), NewPipe[int](0), NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	slow := fn[int, int](func(v int) (int, error) { time.Sleep(20 * time.Millisecond); return v, nil })
	fast := fn[int, int](func(v int) (int, error) { return v, nil })
	c := New(nil)
	c.AddStep(Entry(in, source[int]{[]int{1, 2, 3, 4}}))
	c.AddStep(Route(in, []*Pipe[int]{even, odd}, parity{}))
	c.AddStep(Decorate(even, joined, slow))
	c.AddStep(Decorate(odd, joined, fast))
	c.AddStep(got.sink(joined))
	defer run(t, c)()
	got.wait(t)
	if len(got.flushes) != 1 || got.flushes[0] != 4 {
		t.Errorf("flushes %v (values before each), want one after all 4: %v", got.flushes, got.got)
	}
	select {
	case <-got.done:
		t.Error("a second flush passed the barrier")
	case <-time.After(50 * time.Millisecond):
	}
}

// Parallel: every value done before the flush passes
func TestParallelFlushWaits(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	var busy atomic.Int32
	c := New(nil)
	c.AddStep(Entry(in, source[int]{[]int{1, 2, 3, 4, 5, 6}}))
	c.AddStep(Parallel(3, in, out, fn[int, int](func(v int) (int, error) {
		busy.Add(1)
		time.Sleep(time.Duration(7-v) * 5 * time.Millisecond)
		busy.Add(-1)
		return v, nil
	})))
	c.AddStep(got.sink(out))
	defer run(t, c)()
	got.wait(t)
	if got.flushes[0] != 6 {
		t.Errorf("flush after %d values, want 6", got.flushes[0])
	}
	if busy.Load() != 0 {
		t.Error("a worker was still busy at the flush")
	}
}

// A skip never reaches the error channel; an error does; a sub-chain without its own
// channel uses the one of the chain it runs in
func TestErrorsAndSkips(t *testing.T) {
	errch := make(chan error, 10)
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	boom := errors.New("boom")
	sub := New(nil)
	sub.AddStep(Decorate(in, out, fn[int, int](func(v int) (int, error) {
		switch v {
		case 1:
			return 0, ErrSkippedItem
		case 2:
			return 0, boom
		}
		return v, nil
	})))
	c := New(errch)
	c.AddStep(Entry(in, source[int]{[]int{1, 2, 3}}))
	c.AddStep(sub)
	c.AddStep(got.sink(out))
	defer run(t, c)()
	got.wait(t)
	if len(errch) != 1 || !errors.Is(<-errch, boom) {
		t.Error("want exactly the error, not the skip")
	}
	if !slices.Equal(got.got, []int{3}) {
		t.Errorf("got %v", got.got)
	}
}

type stopCount struct{ n *atomic.Int32 }

func (s stopCount) Decorate(v int) (int, error) { return v, nil }
func (s stopCount) Stop()                       { s.n.Add(1) }

// Cancel ends a step blocked on a send nobody reads; its logic is stopped once
func TestCancelUnblocksAndStopsOnce(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0) // nobody reads out
	var stops atomic.Int32
	c := New(nil)
	c.AddStep(Entry(in, source[int]{[]int{1, 2}}))
	c.AddStep(Decorate[int, int](in, out, stopCount{&stops}))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { c.Process(ctx); close(done) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the chain did not stop")
	}
	if stops.Load() != 1 {
		t.Errorf("Stop called %d times, want 1", stops.Load())
	}
}

// Series: logics one after another; none: values pass
func TestSeries(t *testing.T) {
	for _, n := range []int{0, 1, 3} {
		in, out := NewPipe[int](0), NewPipe[int](0)
		got := newCollect[int]()
		var logics []Decorator[int, int]
		for range n {
			logics = append(logics, fn[int, int](func(v int) (int, error) { return v + 1, nil }))
		}
		c := New(nil)
		c.AddStep(Entry(in, source[int]{[]int{0}}))
		c.AddStep(Series(in, out, logics...))
		c.AddStep(got.sink(out))
		cancel := run(t, c)
		got.wait(t)
		cancel()
		if !slices.Equal(got.got, []int{n}) {
			t.Errorf("%d logics: got %v", n, got.got)
		}
	}
}

// Send: a value from outside the chain reaches it, and is not a writer (no flush is
// waited for from it)
func TestSendFromOutside(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	c := New(nil)
	c.AddStep(Entry(in, source[int]{nil})) // flushes at once
	c.AddStep(Decorate(in, out, fn[int, int](func(v int) (int, error) { return v, nil })))
	c.AddStep(got.sink(out))
	defer run(t, c)()
	got.wait(t)
	if !in.Send(t.Context(), 7) {
		t.Fatal("send failed")
	}
	time.Sleep(20 * time.Millisecond)
	got.mu.Lock()
	defer got.mu.Unlock()
	if !slices.Equal(got.got, []int{7}) {
		t.Errorf("got %v", got.got)
	}
}

// Flush from outside: a sub-chain driven value by value — a skipped value still ends
// with the flush, so the driver knows it is done
func TestFlushFromOutside(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := newCollect[int]()
	c := New(nil)
	c.AddStep(Decorate(in, out, fn[int, int](func(v int) (int, error) {
		if v < 0 {
			return 0, ErrSkippedItem
		}
		return v, nil
	})))
	c.AddStep(got.sink(out))
	defer run(t, c)()
	for _, v := range []int{-1, 5} {
		in.Send(t.Context(), v)
		in.Flush(t.Context())
		got.wait(t)
	}
	if !slices.Equal(got.got, []int{5}) || !slices.Equal(got.flushes, []int{0, 1}) {
		t.Errorf("got %v, flushes %v", got.got, got.flushes)
	}
}
