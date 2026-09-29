package scan

import (
	"context"
	"sync"
)

// progress tells the walker when the work of its last walk is done, so the next
// walk starts only then (a walk is fast, processing may take long; overlapping
// walks would send groups that are still in the chain once more).
//
// Every group the gate lets through ends in exactly one event: an item out of the
// closer, or one error/skip on the processing chain's error channel.
type progress struct {
	mu       sync.Mutex
	inFlight int           // groups passed by the gate and not finished yet
	gated    bool          // the gate has seen every grouper's marker of this walk
	changed  chan struct{} // closed on every change
}

func newProgress() *progress {
	return &progress{changed: make(chan struct{})}
}

func (p *progress) update(f func()) {
	p.mu.Lock()
	f()
	close(p.changed)
	p.changed = make(chan struct{})
	p.mu.Unlock()
}

// passed: the gate let a group through
func (p *progress) passed() { p.update(func() { p.inFlight++ }) }

// finished: a group left the chain (an item, or an error/skip)
func (p *progress) finished() { p.update(func() { p.inFlight-- }) }

// walkGated: the gate has every group of the walk (all markers arrived)
func (p *progress) walkGated() { p.update(func() { p.gated = true }) }

// waitIdle blocks until the walk is gated and nothing is in flight, then resets
// for the next walk. false if ctx is done first.
func (p *progress) waitIdle(ctx context.Context) bool {
	for {
		p.mu.Lock()
		if p.gated && p.inFlight <= 0 {
			p.gated = false
			p.mu.Unlock()
			return true
		}
		changed := p.changed
		p.mu.Unlock()

		select {
		case <-changed:
		case <-ctx.Done():
			return false
		}
	}
}
