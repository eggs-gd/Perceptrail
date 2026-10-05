package model

import (
	"errors"
	"fmt"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	l "github.com/eggs-gd/go-zap-decor"

	"gorm.io/gorm"
)

// ErrNotFound: a lookup found no row (GetFileByPath, GetItemByGuid, …)
var ErrNotFound = gorm.ErrRecordNotFound

// Config: what the model reads of the config — where its data is
type Config interface {
	Database() config.Database
}

// Proxy: the model — the library's data and its rules (ItemsApi, FilesApi, MetaApi
// and the import's rules); one per run, opened in main and passed to who uses it.
//
// Reads go to a pool of read-only connections; writes to the one writer: a write
// method's body is its rule (wrapped in write / written), run whole — read, decide,
// write — in the writer's transaction under its own savepoint; its result comes
// back after the commit.
type Proxy struct {
	logger *l.Logger
	db     *gorm.DB // the readers' pool; in a rule, the writer's transaction
	inRule bool     // bound to the writer's transaction: writes run right there

	writer *writer
	rules  *pubsub.Op[*gorm.DB, func(*Proxy) error, struct{}]
	writes *gorm.DB // the writer's connection (closed with the model)
}

// Open connects to the database of the config — the writer, then the readers —
// and migrates its schema
func Open(cfg Config, logger *l.Logger) (*Proxy, error) {
	logger.Info("Database opening", l.String("driver", cfg.Database().Driver), l.String("name", cfg.Database().Name))
	writes, err := connect(cfg, false, logger)
	if err != nil {
		return nil, err
	}
	if err := writes.AutoMigrate(&dto.ItemDto{}, &dto.FileDto{}, &dto.MetaDto{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	reads, err := connect(cfg, true, logger)
	if err != nil {
		return nil, err
	}
	p := &Proxy{logger: logger, db: reads, writes: writes, writer: newWriter(writes)}
	p.rules = pubsub.New(p.writer, pubsub.Frame, func(tx *gorm.DB, rule func(*Proxy) error) (struct{}, error) {
		return struct{}{}, p.run(tx, rule)
	})
	return p, nil
}

// connect: a connection pool of the config's database, for the writer or the
// readers
func connect(cfg Config, read bool, logger *l.Logger) (*gorm.DB, error) {
	d, dialector, err := dialectorOf(cfg.Database(), read)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: newLogger(logger)})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := d.tune(db, read); err != nil {
		return nil, fmt.Errorf("configure: %w", err)
	}
	return db, nil
}

// write: the rule runs in the writer's transaction and this returns after the
// commit; called inside a rule, it runs right there, in the same transaction (the
// writer is busy with the rule that called it)
func (p *Proxy) write(rule func(q *Proxy) error) error {
	if p.inRule {
		return rule(p)
	}
	_, err := p.rules.Do(rule)
	return err
}

// written: write, for a rule with a result
func written[R any](p *Proxy, rule func(q *Proxy) (R, error)) (R, error) {
	var r R
	err := p.write(func(q *Proxy) (err error) {
		r, err = rule(q)
		return err
	})
	return r, err
}

// run: one rule in the writer's transaction, under its own savepoint — a failing
// rule (an error, a panic) rolls back alone, the batch commits
func (p *Proxy) run(tx *gorm.DB, rule func(*Proxy) error) (err error) {
	if err := tx.SavePoint("rule").Error; err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("model: a rule panicked: %v", r)
		}
		if err != nil {
			tx.RollbackTo("rule")
		}
		tx.Exec("RELEASE SAVEPOINT rule")
	}()
	return rule(&Proxy{logger: p.logger, db: tx, inRule: true})
}

// Close: the writer writes what is queued and stops; the readers close, then the
// writer's connection — last, so it merges the journal into the file and removes it
// (a read-only connection closing last cannot)
func (p *Proxy) Close() error {
	p.writer.close()
	readers := closeDB(p.db)
	return errors.Join(readers, closeDB(p.writes))
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
