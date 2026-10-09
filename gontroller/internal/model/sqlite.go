package model

import (
	"net/url"
	"path/filepath"

	"perceptrail/gontroller/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// readers: the read pool's connections
const readers = 4

var sqliteDriver = driver{
	dialector: func(cfg config.Database, read bool) (gorm.Dialector, error) {
		dsn := sqliteDSN(cfg.Name)
		if read {
			dsn += "&mode=ro" // the readers never write: only the writer does
		}
		return sqlite.Open(dsn), nil
	},
	tune: func(db *gorm.DB, read bool) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		// The writer: one connection (SQLite has one writer; its goroutine owns it).
		// The readers: a pool — WAL lets them read beside the writer.
		conns := 1
		if read {
			conns = readers
		}
		sqlDB.SetMaxOpenConns(conns)
		sqlDB.SetMaxIdleConns(conns)
		return nil
	},
}

// sqliteDSN builds a SQLite URI for path. The path is percent-encoded: a raw '#',
// '?' or '%' (e.g. in data_dir) would otherwise change which file is opened.
// WAL + busy_timeout: the writer writes while the readers read. synchronous=NORMAL:
// with WAL a power loss may lose the last transactions, never corrupts — and our
// data comes back from the disk anyway.
func sqliteDSN(path string) string {
	u := url.URL{Path: filepath.ToSlash(path)}
	return "file:" + u.EscapedPath() + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL&_fk=1"
}
