package model

import (
	"errors"
	"sync"

	pubsub "github.com/eggs-gd/go-pub-sub"

	"gorm.io/gorm"
)

// errClosed: a write submitted after the model was closed
var errClosed = errors.New("model: closed")

// maxBatch: the most jobs one transaction takes — a long one would hold back a
// Now job queued behind it
const maxBatch = 512

// writer: the model's one writer, the executor of its write operations (pubsub).
// One goroutine owns the write connection: it takes a job (Now first, then Frame,
// then Idle), takes whatever else is queued already, runs them all in one
// transaction — each operation under its own savepoint (see Proxy.write) — commits,
// then ends them: their results go out after the commit.
//
// Not obvious:
//   - Nothing waits for a batch to fill yet: every caller still waits for its own
//     result (Do), so a deadline would be paid per call. Batches come from callers
//     writing at once; the classes' deadlines come with asynchronous callers.
//   - A full lane makes Enqueue wait (backpressure); after Close it fails the job
//     instead.
type writer struct {
	db    *gorm.DB
	lanes [3]chan pubsub.Job[*gorm.DB] // by class: Now, Frame, Idle
	quit  chan struct{}                // closed: no job is taken any more (closed())
	done  chan struct{}

	mu sync.RWMutex // senders read-lock; quit is closed under the write lock, so no job slips in after it
}

// Enqueue: the job into its class's lane (waiting for room when it is full); once
// closed, the job fails
func (w *writer) Enqueue(job pubsub.Job[*gorm.DB]) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed() {
		job.Done(errClosed)
		return
	}
	w.lanes[lane(job.Class())] <- job
}

func newWriter(db *gorm.DB) *writer {
	w := &writer{db: db, quit: make(chan struct{}), done: make(chan struct{})}
	for i := range w.lanes {
		w.lanes[i] = make(chan pubsub.Job[*gorm.DB], maxBatch)
	}
	go w.loop()
	return w
}

// closed: the writer takes no job any more
func (w *writer) closed() bool {
	select {
	case <-w.quit:
		return true
	default:
		return false
	}
}

// close: no job is taken any more; the queued ones are written, then the writer
// stops
func (w *writer) close() {
	w.mu.Lock()
	if !w.closed() {
		close(w.quit)
	}
	w.mu.Unlock()
	<-w.done
}

func (w *writer) loop() {
	defer close(w.done)
	for {
		first, ok := w.next()
		if !ok {
			return
		}
		batch := w.gather(first)
		err := w.db.Transaction(func(tx *gorm.DB) error {
			for _, job := range batch {
				job.Run(tx) // an operation's error rolls back to its own savepoint
			}
			return nil
		})
		for _, job := range batch {
			job.Done(err)
		}
	}
}

// next: the next job, Now first; false once closed with nothing left
func (w *writer) next() (pubsub.Job[*gorm.DB], bool) {
	if job, ok := w.queued(); ok {
		return job, true
	}
	select {
	case job := <-w.lanes[0]:
		return job, true
	case job := <-w.lanes[1]:
		return job, true
	case job := <-w.lanes[2]:
		return job, true
	case <-w.quit:
		return w.queued() // what was queued before the close is still written
	}
}

// gather: first and what is queued already, Now first, up to maxBatch
func (w *writer) gather(first pubsub.Job[*gorm.DB]) []pubsub.Job[*gorm.DB] {
	batch := []pubsub.Job[*gorm.DB]{first}
	for len(batch) < maxBatch {
		job, ok := w.queued()
		if !ok {
			break
		}
		batch = append(batch, job)
	}
	return batch
}

// queued: a job already waiting, the most urgent lane first
func (w *writer) queued() (pubsub.Job[*gorm.DB], bool) {
	for _, l := range w.lanes {
		select {
		case job := <-l:
			return job, true
		default:
		}
	}
	return nil, false
}

// lane: a class's lane (an unknown class counts as Idle)
func lane(c pubsub.Class) int {
	if c < pubsub.Now || c > pubsub.Idle {
		return int(pubsub.Idle)
	}
	return int(c)
}
