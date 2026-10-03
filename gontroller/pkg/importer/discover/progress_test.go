package discover

import (
	"context"
	"testing"
	"time"
)

// The walker walks again only after the previous walk's work is done
func TestWalkerRepeatsAfterIdle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/a.jpg")

	m := newTestMonitor(t, root)
	m.interval = time.Millisecond
	m.progress = NewProgress()
	chin := make(chan inType)
	go m.Start(chin, m.ctx)

	walks := 0
	deadline := time.After(5 * time.Second)
	for walks < 2 {
		select {
		case in := <-chin:
			if in.done == nil {
				continue
			}
			walks++
			if walks == 1 {
				// A group of the walk is still being processed: no second walk yet
				m.progress.Passed()
				m.progress.WalkGated()
				select {
				case in := <-chin:
					t.Fatalf("walked again while busy: %+v", in)
				case <-time.After(50 * time.Millisecond):
				}
				m.progress.Finished()
			} else {
				m.progress.WalkGated()
			}
		case <-deadline:
			t.Fatalf("only %d walks", walks)
		}
	}
}

func idleSoon(p *Progress) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	return p.WaitIdle(ctx)
}

// Idle only when the walk is gated and every passed group has finished
func TestProgressWaitIdle(t *testing.T) {
	p := NewProgress()
	p.Passed()
	p.Passed()
	if idleSoon(p) {
		t.Fatal("idle before the gate saw the whole walk")
	}
	p.WalkGated()
	p.Finished()
	if idleSoon(p) {
		t.Fatal("idle with a group still in flight")
	}
	p.Finished()
	if !idleSoon(p) {
		t.Fatal("not idle when all is done")
	}
	if idleSoon(p) {
		t.Fatal("waitIdle must reset for the next walk")
	}
}
