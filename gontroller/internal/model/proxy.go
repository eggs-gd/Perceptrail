package model

import (
	"errors"
	"fmt"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model/dto"

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
// Reads go to a pool of read-only connections; writes to the one writer: each write
// (writes.go) runs whole — read, decide, write — in the writer's transaction, under
// its own savepoint; its result comes back after the commit — waited for by the
// public method of its name, or by subscription to its topic.
type Proxy struct {
	query  // reads, over the readers' pool
	logger *l.Logger

	writer   *writer
	commands          // every write as a command (writes.go)
	writes   *gorm.DB // the writer's connection (closed with the model)
}

// query: the model's reads over a connection — the readers' pool (Proxy), or a
// write's transaction (tx)
type query struct {
	db *gorm.DB
}

// tx: what a write runs on — the reads and the writes themselves, over the writer's
// transaction. It has no public writes: a write calls other writes directly, in the
// same transaction (a public write would wait for the writer that runs it).
type tx struct {
	query
	logger *l.Logger
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
	p := &Proxy{query: query{reads}, logger: logger, writes: writes, writer: newWriter(writes)}
	p.commands = newCommands(p)
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

// run: one write in the writer's transaction, under its own savepoint — a failing
// write (an error, a panic) rolls back alone, the batch commits
func (p *Proxy) run(db *gorm.DB, write func(*tx) error) (err error) {
	if err := db.SavePoint("write").Error; err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("model: a write panicked: %v", r)
		}
		if err != nil {
			db.RollbackTo("write")
		}
		db.Exec("RELEASE SAVEPOINT write")
	}()
	return write(&tx{query: query{db}, logger: p.logger})
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
