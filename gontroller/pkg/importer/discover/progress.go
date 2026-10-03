package discover

import (
	"context"
	"sync"
	"time"
)

// Walk describes a finished walk. Deletions may be derived from it only if the walk
// was complete: a cancelled walk or an unreadable root says nothing about which
// files are gone.
type Walk struct {
	Root    string
	Started time.Time
	// The walk reached the end
	Complete bool
	// Files seen (before grouping and filtering)
	Files int
	// Directories that could not be read: their files are not "deleted"
	Unreadable []string
}

// Progress: the last walk, and whether its work is done. The walker flushes the
// chain after a walk; the flush passes every step after the values before it, so
// when it reaches the end of the chain (Done) every group of the walk is through. The
// next walk starts only then: walks never overlap, no group is in the chain twice.
type Progress struct {
	mu   sync.Mutex
	last Walk
	done chan struct{}
}

func NewProgress() *Progress {
	done := make(chan struct{})
	close(done)
	return &Progress{done: done}
}

// Walked: a walk finished (the walker, before it flushes)
func (p *Progress) Walked(w Walk) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.last = w
	p.done = make(chan struct{})
}

// Last: the last finished walk (the gate's deletions)
func (p *Progress) Last() Walk {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

// Done: the walk's flush reached the end of the chain
func (p *Progress) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
	default:
		close(p.done)
	}
}

// WaitIdle blocks until the last walk's work is done; false if ctx ends first
func (p *Progress) WaitIdle(ctx context.Context) bool {
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}
