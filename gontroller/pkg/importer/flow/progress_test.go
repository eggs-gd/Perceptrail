package flow

import (
	"context"
	"testing"
	"time"
)

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
