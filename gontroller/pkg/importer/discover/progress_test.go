package discover

import (
	"context"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/providers"
)

func idleSoon(p *Progress) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	return p.WaitIdle(ctx)
}

// Idle until a walk is recorded; then only once its flush reached the end (Done); a
// second Done is harmless
func TestProgressWaitIdle(t *testing.T) {
	p := NewProgress()
	if !idleSoon(p) {
		t.Error("no walk yet: idle")
	}
	p.Walked(Walk{Root: "/lib", Files: 3})
	if idleSoon(p) {
		t.Error("idle before the walk's flush reached the end")
	}
	if p.Last().Files != 3 {
		t.Errorf("last walk %+v", p.Last())
	}
	p.Done()
	p.Done()
	if !idleSoon(p) {
		t.Error("not idle after Done")
	}
}

// emitter: what the walker sends, flushes counted
type emitter struct{ flushes chan struct{} }

func (emitter) Emit(providers.Found) bool { return true }
func (e emitter) Flush() bool             { e.flushes <- struct{}{}; return true }

// The walker walks again only after the previous walk's work is done
func TestWalkerRepeatsAfterIdle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/a.jpg")
	m := newTestWalker(root)
	m.interval = time.Millisecond
	out := emitter{flushes: make(chan struct{})}
	go m.Run(t.Context(), out)

	select {
	case <-out.flushes:
	case <-time.After(5 * time.Second):
		t.Fatal("no first walk")
	}
	select {
	case <-out.flushes:
		t.Fatal("walked again while the first walk was in the chain")
	case <-time.After(50 * time.Millisecond):
	}
	m.progress.Done() // the first walk's flush reached the end
	select {
	case <-out.flushes:
	case <-time.After(5 * time.Second):
		t.Fatal("no second walk after the first was done")
	}
}
