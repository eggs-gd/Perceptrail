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

type fn[Ti, To any] func(Ti) (To, error)

func (f fn[Ti, To]) Decorate(v Ti) (To, error) { return f(v) }

// collect: the values an end consumed
type collect[T any] struct {
	mu  sync.Mutex
	got []T
}

func (c *collect[T]) Consume(v T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, v)
	return nil
}

func (c *collect[T]) values() []T {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.got)
}

// count: a pass emits 1..n
type count struct{ n int }

func (c count) Start(_ context.Context, emit func(int) bool) error {
	for i := 1; i <= c.n; i++ {
		emit(i)
	}
	return nil
}

// values: a pass emits these
type values []int

func (vs values) Start(_ context.Context, emit func(int) bool) error {
	for _, v := range vs {
		emit(v)
	}
	return nil
}

func start(t *testing.T, c *Chain) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go c.Process(ctx)
}

func pass(t *testing.T, c *Chain) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if !c.Run(ctx) {
		t.Fatal("the flush never reached the ends")
	}
}

// A pass: the entry's values, then its flush; Run returns when the flush reached the
// end — every value went through by then; the next pass the same
func TestPasses(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := &collect[int]{}
	c := New(nil)
	c.AddStep(Entry[int](in, count{3}))
	c.AddStep(Decorate(in, out, fn[int, int](func(v int) (int, error) { return v * 10, nil })))
	c.AddStep(End[int](out, got))
	start(t, c)
	for range 2 { // a pass when the owner wants
		pass(t, c)
	}
	if !slices.Equal(got.values(), []int{10, 20, 30, 10, 20, 30}) {
		t.Errorf("got %v", got.values())
	}
}

// holder keeps every value until the flush
type holder struct{ held []int }

func (h *holder) Decorate(v int) (int, error) { h.held = append(h.held, v); return 0, ErrSkippedItem }
func (h *holder) Flush() ([]int, error)       { out := h.held; h.held = nil; return out, nil }

// A Flusher's values go out on the flush, before it
func TestFlusher(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := &collect[int]{}
	c := New(nil)
	c.AddStep(Entry[int](in, count{2}))
	c.AddStep(Decorate[int, int](in, out, &holder{}))
	c.AddStep(End[int](out, got))
	start(t, c)
	pass(t, c)
	if !slices.Equal(got.values(), []int{1, 2}) {
		t.Errorf("got %v", got.values())
	}
}

type parity struct{}

func (parity) Route(v int) (int, error) { return v % 2, nil }

// Route: a value to one output, the flush to all; a join passes the flush once
// every branch flushed
func TestRouteJoinAndEnds(t *testing.T) {
	in, even, odd := NewPipe[int](0), NewPipe[int](0), NewPipe[int](0)
	joined, tail := NewPipe[int](0), NewPipe[int](0)
	slow := fn[int, int](func(v int) (int, error) { time.Sleep(10 * time.Millisecond); return v, nil })
	same := fn[int, int](func(v int) (int, error) { return v, nil })
	a := &collect[int]{}
	c := New(nil)
	c.AddStep(Entry[int](in, count{4}))
	c.AddStep(Route(in, []*Pipe[int]{even, odd}, parity{}))
	c.AddStep(Decorate(even, joined, slow))
	c.AddStep(Decorate(odd, joined, same))
	c.AddStep(Decorate(joined, tail, same))
	c.AddStep(End[int](tail, a))
	start(t, c)
	pass(t, c)
	if got := a.values(); len(got) != 4 {
		t.Errorf("the join passed the flush before every branch: %v", got)
	}
}

// Parallel: the flush waits for every value before it
func TestParallel(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0)
	got := &collect[int]{}
	var busy atomic.Int32
	c := New(nil)
	c.AddStep(Entry[int](in, count{6}))
	c.AddStep(Parallel(3, in, out, fn[int, int](func(v int) (int, error) {
		busy.Add(1)
		defer busy.Add(-1)
		time.Sleep(time.Duration(7-v) * 3 * time.Millisecond)
		return v, nil
	})))
	c.AddStep(End[int](out, got))
	start(t, c)
	pass(t, c)
	if len(got.values()) != 6 || busy.Load() != 0 {
		t.Errorf("got %v, busy %d at the flush", got.values(), busy.Load())
	}
}

// A skip is never an error; an error is; a sub-chain uses its parent's channel (and
// its passes)
func TestErrors(t *testing.T) {
	errch := make(chan error, 10)
	in, out := NewPipe[int](0), NewPipe[int](0)
	boom := errors.New("boom")
	sub := New(nil)
	sub.AddStep(Entry[int](in, values{1, 2, 3}))
	sub.AddStep(Decorate(in, out, fn[int, int](func(v int) (int, error) {
		switch v {
		case 1:
			return 0, ErrSkippedItem
		case 2:
			return 0, boom
		}
		return v, nil
	})))
	got := &collect[int]{}
	c := New(errch)
	c.AddStep(sub)
	c.AddStep(End[int](out, got))
	start(t, c)
	pass(t, c)
	if len(errch) != 1 || !errors.Is(<-errch, boom) || !slices.Equal(got.values(), []int{3}) {
		t.Errorf("errors %d, got %v", len(errch), got.values())
	}
}

type stopCount struct{ n *atomic.Int32 }

func (s stopCount) Decorate(v int) (int, error) { return v, nil }
func (s stopCount) Stop()                       { s.n.Add(1) }

// Cancel ends a step blocked on a send nobody reads; its logic is stopped once
func TestCancel(t *testing.T) {
	in, out := NewPipe[int](0), NewPipe[int](0) // nobody reads out
	var stops atomic.Int32
	c := New(nil)
	c.AddStep(Entry[int](in, values{1}))
	c.AddStep(Decorate[int, int](in, out, stopCount{&stops}))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { c.Process(ctx); close(done) }()
	ran := make(chan bool)
	go func() { ran <- c.Run(ctx) }()
	time.Sleep(20 * time.Millisecond) // the value is stuck in the step
	cancel()
	if <-ran {
		t.Error("a cancelled pass reported done")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the chain did not stop")
	}
	if stops.Load() != 1 {
		t.Errorf("Stop called %d times", stops.Load())
	}
}
