package chain

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type mockExpander struct {
	expandFunc func(int) ([]int, error)
	mu         sync.Mutex
	stopped    bool
}

func (m *mockExpander) Expand(i int) ([]int, error) { return m.expandFunc(i) }
func (m *mockExpander) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

func TestExpander(t *testing.T) {
	chin := make(chan int)
	chout := make(chan int)
	cherr := make(chan error, 1)
	failure := errors.New("bad input")

	// n -> n copies of n; 0 -> nothing; -1 -> an error
	mock := &mockExpander{expandFunc: func(i int) ([]int, error) {
		if i < 0 {
			return nil, failure
		}
		out := make([]int, i)
		for k := range out {
			out[k] = i
		}
		return out, nil
	}}

	exp := NewExpander(chin, chout, mock)
	exp.setErrorChannel(cherr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { exp.Process(ctx); close(done) }()

	go func() {
		for _, i := range []int{2, 0, -1, 3} {
			chin <- i
		}
		close(chin)
	}()

	var got []int
	for len(got) < 5 {
		select {
		case o := <-chout:
			got = append(got, o)
		case err := <-cherr:
			if !errors.Is(err, failure) {
				t.Fatalf("unexpected error %v", err)
			}
		}
	}
	<-done

	want := []int{2, 2, 3, 3, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if !mock.stopped {
		t.Error("Stop was not called when the input closed")
	}
}
