package model

import (
	"net/url"
	"path/filepath"

	"perceptrail/gontroller/pkg/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var sqliteDriver = driver{
	dialector: func(cfg config.Database) (gorm.Dialector, error) {
		return sqlite.Open(sqliteDSN(cfg.Name)), nil
	},
	tune: func(db *gorm.DB) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		// One connection: avoids SQLite "database is locked" under concurrent GORM pools.
		// StreamAllItems uses short keyset pages, so the conn is released while the client is written to.
		// Never open a transaction and query db (not tx) inside it — that would deadlock on this single conn.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		return nil
	},
}

// sqliteDSN builds a SQLite URI for path. The path is percent-encoded: a raw '#',
// '?' or '%' (e.g. in data_dir) would otherwise change which file is opened.
// WAL + busy_timeout: the importer writes while the /items stream reads.
func sqliteDSN(path string) string {
	u := url.URL{Path: filepath.ToSlash(path)}
	return "file:" + u.EscapedPath() + "?_busy_timeout=5000&_journal_mode=WAL&_fk=1"
}
