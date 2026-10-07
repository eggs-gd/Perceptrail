package model

import (
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	"github.com/eggs-gd/perceplib/api"
	"gorm.io/gorm"
)

// The model's domain events: facts of the library others react to, not a write's
// raw result. A write emits them inside its transaction; the writer publishes them
// only after the commit, in the order they were emitted — no one hears of a write
// that is not there.
//
// Not obvious:
//   - A write rolled back to its savepoint takes its events with it; a commit that
//     fails drops the whole batch's.
//   - Listeners are called on the writer's goroutine: they hand the work off and
//     return (the database is the truth, an event only says "look").

// ItemPublished: an item went through the cheap stage (Visible or Waiting) — the
// expensive stage may have work for it
type ItemPublished struct {
	GUID  api.GUID
	State dto.ItemState
}

// events: the model's topics
type events struct {
	published pubsub.Topic[ItemPublished]
}

// batch: what one transaction of the writer carries — its connection and the events
// its writes emitted, sent after the commit
type batch struct {
	db     *gorm.DB
	outbox []func()
}

// Published: items through the cheap stage, after the commit
func (p *Proxy) Published() *pubsub.Topic[ItemPublished] { return &p.events.published }

// send: the batch's events, once it is committed
func (b *batch) send() {
	for _, publish := range b.outbox {
		publish()
	}
}

// emit: an event to publish once the transaction commits
func emit[E any](t *tx, topic *pubsub.Topic[E], e E) {
	t.batch.outbox = append(t.batch.outbox, func() { topic.Publish(e) })
}
