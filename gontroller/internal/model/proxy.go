package model

import (
	"errors"
	"fmt"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"

	"gorm.io/gorm"
)

// Config: what the model reads of the config — where its data is
type Config interface {
	Database() config.Database
}

// ErrNotFound: a lookup found no row (GetFileByPath, GetItemByGUID, …)
var ErrNotFound = gorm.ErrRecordNotFound

// Proxy: the model — the library's data and its rules (items.go, identity.go,
// flow.go, files.go, meta.go); one per run, opened in main and passed to who uses it.
//
// Reads go to a pool of read-only connections; writes to the one writer: each write
// (command.go) runs whole — read, decide, write — in the writer's transaction, under
// its own savepoint; its result comes back after the commit — waited for by the
// public method of its name, or through its command.
type Proxy struct {
	query  // reads, over the readers' pool
	logger *l.Logger

	writer *writer
	events events // domain events, published after the commit (events.go)
	// every write as a command (command.go), each file its own
	itemCommands
	identityCommands
	flowCommands
	fileCommands
	metaCommands
	workCommands
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
	batch  *batch  // the writer's transaction: its events go out after its commit
	events *events // the topics a write emits to
}

// Open connects to the database of the config — the writer, then the readers —
// and migrates its schema
func Open(cfg Config, logger *l.Logger) (*Proxy, error) {
	logger.Info("Database opening", l.String("driver", cfg.Database().Driver), l.String("name", cfg.Database().Name))
	writes, err := connect(cfg, false, logger)
	if err != nil {
		return nil, err
	}
	// Renditions were kept per render version for a while: those rows go, and render
	// makes them again
	if writes.Migrator().HasColumn(&dto.RenditionDto{}, "version") {
		if err := writes.Migrator().DropTable(&dto.RenditionDto{}); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
		if err := writes.Where("slug = ?", "render").Delete(&dto.WorkDto{}).Error; err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	if err := writes.AutoMigrate(&dto.ItemDto{}, &dto.FileDto{}, &dto.MetaDto{}, &dto.WorkDto{}, &dto.RenditionDto{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	// An ignored group was marked "-" before the GUID had a nil value of its own
	if err := writes.Model(&dto.FileDto{}).Where("linked_to = ?", "-").Update("linked_to", api.NilGUID).Error; err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	reads, err := connect(cfg, true, logger)
	if err != nil {
		return nil, err
	}
	p := &Proxy{query: query{reads}, logger: logger, writer: newWriter(writes)}
	p.itemCommands, p.identityCommands, p.flowCommands = newItemCommands(p), newIdentityCommands(p), newFlowCommands(p)
	p.fileCommands, p.metaCommands, p.workCommands = newFileCommands(p), newMetaCommands(p), newWorkCommands(p)
	return p, nil
}

// Close: the writer writes what is queued and stops; the readers close, then the
// writer's connection — last, so it merges the journal into the file and removes it
// (a read-only connection closing last cannot)
func (p *Proxy) Close() error {
	p.writer.close()
	readers := closeDB(p.db)
	return errors.Join(readers, closeDB(p.writer.db))
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
func (p *Proxy) run(b *batch, write func(*tx) error) (err error) {
	db, emitted := b.db, len(b.outbox)
	if err := db.SavePoint("write").Error; err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("model: a write panicked: %v", r)
		}
		if err != nil {
			db.RollbackTo("write")
			b.outbox = b.outbox[:emitted] // its events go with it
		}
		db.Exec("RELEASE SAVEPOINT write")
	}()
	return write(&tx{query: query{db}, logger: p.logger, batch: b, events: &p.events})
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
