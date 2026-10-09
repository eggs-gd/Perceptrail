package model

import (
	"fmt"

	"perceptrail/gontroller/internal/config"

	"gorm.io/gorm"
)

// driver opens one kind of database — for the writer, or for the readers (read:
// a read-only pool). tune adjusts the connection pool after gorm.Open (e.g. sqlite:
// one connection to write).
type driver struct {
	dialector func(cfg config.Database, read bool) (gorm.Dialector, error)
	tune      func(db *gorm.DB, read bool) error
}

var drivers = map[string]driver{
	config.DriverSQLite:   sqliteDriver,
	config.DriverPostgres: postgresDriver,
}

// dialectorOf: the database's driver, or why it cannot be opened (unknown, not
// implemented yet)
func dialectorOf(cfg config.Database, read bool) (driver, gorm.Dialector, error) {
	d, ok := drivers[cfg.Driver]
	if !ok {
		return driver{}, nil, fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
	dialector, err := d.dialector(cfg, read)
	return d, dialector, err
}
