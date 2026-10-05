package model

import (
	pubsub "github.com/eggs-gd/go-pub-sub"
	"gorm.io/gorm"
)

// Every write of the model is a command (an Op) run by the writer, with two faces,
// in pairs: the public method of its name submits and waits for its own result;
// its Command submits and goes on — in the view of its shape (a Message without a
// result, a Signal without an argument). A write takes one argument and gives one
// result (pubsub.None where it has none): its method on tx is the command's
// function as it is. Each file keeps its own commands (itemCommands in items.go…).

// op: a command of the model, run by the writer
type op[A, R any] = *pubsub.Op[*gorm.DB, A, R]

// command: a write as a command — it runs in the writer's transaction, under its own
// savepoint (run)
func command[A, R any](p *Proxy, class pubsub.Class, fn func(t *tx, arg A) (R, error)) op[A, R] {
	return pubsub.New(p.writer, class, func(db *gorm.DB, arg A) (R, error) {
		var r R
		err := p.run(db, func(t *tx) (err error) {
			r, err = fn(t, arg)
			return err
		})
		return r, err
	})
}
