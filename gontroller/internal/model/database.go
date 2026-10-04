package model

import (
	"fmt"

	"perceptrail/gontroller/internal/config"

	"gorm.io/gorm"
)

// driver opens one kind of database. tune adjusts the connection pool after
// gorm.Open (e.g. sqlite needs a single connection).
type driver struct {
	dialector func(cfg config.Database) (gorm.Dialector, error)
	tune      func(db *gorm.DB) error
}

var drivers = map[string]driver{
	config.DriverSQLite:   sqliteDriver,
	config.DriverPostgres: postgresDriver,
}

// dialectorOf: the database's driver, or why it cannot be opened (unknown, not
// implemented yet)
func dialectorOf(cfg config.Database) (driver, gorm.Dialector, error) {
	d, ok := drivers[cfg.Driver]
	if !ok {
		return driver{}, nil, fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
	dialector, err := d.dialector(cfg)
	return d, dialector, err
}
