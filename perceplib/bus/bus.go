// Package bus: write operations as topics. A write rule is an Op: a caller submits
// an argument and gets an ID at once; the host's Executor runs the rule (for a
// database: in its one writer's transaction, batched by class) and, after the
// commit, the result goes to every subscriber of the rule — each picks its own by
// ID. Do submits and waits for its own result, for callers that may block.
//
// The bus does not know what runs it: T is the host's transaction type, seen only
// by the host and its Executor; a step sees a Topic. Stdlib only — every
// dependency of perceplib is one each plugin must match.
//
// Not obvious:
//   - Delivery never blocks the executor: a subscriber whose buffer is full misses
//     the result (Dropped counts it). A lost subscription is its owner's loss.
//   - A result comes only after the commit: what its receiver reads next is there.
//     If the commit fails, the rule's result is replaced by the commit's error.
//   - Results reach a subscriber in the order the executor finished them — for one
//     class, the order they were submitted; across subscribers nothing is ordered.
package bus

import (
	"sync"
	"sync/atomic"
)

// ID: an operation of an Op, unique within it
type ID uint64

// Class: how long a rule tolerates waiting for a batch — a property of the rule,
// not of its caller
type Class int

const (
	Now   Class = iota // never waits; taken before the others
	Frame              // waits a frame (~33 ms) or a full batch
	Idle               // waits longer, only while nothing else does
)

// Job: one submitted operation as an executor sees it — its class, the rule to run
// in a transaction, and the end once the transaction is over
type Job[T any] interface {
	Class() Class
	// Run: the rule, inside the transaction tx (its result stays in the job)
	Run(tx T)
	// Done: after the commit, or the rollback (commitErr); delivers the result
	Done(commitErr error)
}

// Executor: who runs the jobs — the host's (the model's writer)
type Executor[T any] interface {
	Enqueue(job Job[T])
}

// Topic: a write rule as a caller sees it, whatever runs it
type Topic[A, R any] interface {
	Submit(arg A) ID
	Subscribe(buffer int) *Sub[R]
	Do(arg A) (R, error)
}

// Result: what an operation gave — its ID, its value or its error
type Result[R any] struct {
	ID    ID
	Value R
	Err   error
}

// Op: one write rule, a topic of its results
type Op[T, A, R any] struct {
	exec  Executor[T]
	class Class
	rule  func(tx T, arg A) (R, error)
	last  atomic.Uint64

	mu   sync.Mutex
	subs map[*Sub[R]]struct{}
}

var _ Topic[int, int] = (*Op[any, int, int])(nil)

// New: a write rule of this class, run by exec
func New[T, A, R any](exec Executor[T], class Class, rule func(tx T, arg A) (R, error)) *Op[T, A, R] {
	return &Op[T, A, R]{exec: exec, class: class, rule: rule, subs: map[*Sub[R]]struct{}{}}
}

// Submit: queued, returns its ID at once; the result goes to the subscribers
func (o *Op[T, A, R]) Submit(arg A) ID {
	id := ID(o.last.Add(1))
	o.exec.Enqueue(&job[T, A, R]{op: o, id: id, arg: arg})
	return id
}

// Do: submitted, and waited for — its own result (the subscribers get it too)
func (o *Op[T, A, R]) Do(arg A) (R, error) {
	reply := make(chan Result[R], 1)
	o.exec.Enqueue(&job[T, A, R]{op: o, id: ID(o.last.Add(1)), arg: arg, reply: reply})
	r := <-reply
	return r.Value, r.Err
}

// Subscribe: every result of this rule from now on, buffered; filter yours by ID
func (o *Op[T, A, R]) Subscribe(buffer int) *Sub[R] {
	s := &Sub[R]{c: make(chan Result[R], buffer), op: o}
	s.C = s.c
	o.mu.Lock()
	o.subs[s] = struct{}{}
	o.mu.Unlock()
	return s
}

// publish: a result to every subscriber, never waiting for one
func (o *Op[T, A, R]) publish(r Result[R]) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for s := range o.subs {
		select {
		case s.c <- r:
		default:
			s.dropped.Add(1)
		}
	}
}

func (o *Op[T, A, R]) unsubscribe(s *Sub[R]) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.subs[s]; ok {
		delete(o.subs, s)
		close(s.c) // under the lock: no publish sends after it
	}
}

// Sub: a subscription to a rule's results
type Sub[R any] struct {
	C       <-chan Result[R]
	c       chan Result[R]
	op      interface{ unsubscribe(*Sub[R]) }
	dropped atomic.Uint64
}

// Close: no more results; C closes (what it buffered can still be read)
func (s *Sub[R]) Close() { s.op.unsubscribe(s) }

// Dropped: the results this subscription missed, its buffer full
func (s *Sub[R]) Dropped() uint64 { return s.dropped.Load() }

// job: an operation of an Op — its argument and, once run, its result
type job[T, A, R any] struct {
	op    *Op[T, A, R]
	id    ID
	arg   A
	value R
	err   error
	reply chan Result[R] // Do's own; nil for Submit
}

func (j *job[T, A, R]) Class() Class { return j.op.class }

func (j *job[T, A, R]) Run(tx T) { j.value, j.err = j.op.rule(tx, j.arg) }

func (j *job[T, A, R]) Done(commitErr error) {
	r := Result[R]{ID: j.id, Value: j.value, Err: j.err}
	if commitErr != nil && j.err == nil {
		var zero R
		r.Value, r.Err = zero, commitErr
	}
	j.op.publish(r) // Do's too: a topic is every result of its rule
	if j.reply != nil {
		j.reply <- r
	}
}

// Inline: an executor that runs a job at once on the caller's goroutine, with the
// zero transaction — for tests, and for hosts with nothing to batch
type Inline[T any] struct{}

func (Inline[T]) Enqueue(j Job[T]) {
	var tx T
	j.Run(tx)
	j.Done(nil)
}
