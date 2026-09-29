package model

import (
	"fmt"

	"gorm.io/gorm"
)

// Database drivers (DBConfig.Driver)
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// DBConfig is the `database` section of the config. GORM hides the driver, so
// the rest of the model does not depend on it.
type DBConfig struct {
	Driver string `yaml:"driver"`
	// sqlite: the database file; server drivers: the database name
	Name string `yaml:"name"`
	// Server drivers only; sqlite ignores them
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`
}

// driver opens one kind of database. tune adjusts the connection pool after
// gorm.Open (e.g. sqlite needs a single connection).
type driver struct {
	dialector func(cfg DBConfig) (gorm.Dialector, error)
	tune      func(db *gorm.DB) error
}

var drivers = map[string]driver{
	DriverSQLite:   sqliteDriver,
	DriverPostgres: postgresDriver,
}

// dbConfig has no default: the caller owns the location (app.Config.Database)
var dbConfig *DBConfig

// Configure selects the database for every proxy. Call it before the first
// NewProxy; it fails fast on an unknown or unimplemented driver.
func Configure(cfg DBConfig) error {
	d, ok := drivers[cfg.Driver]
	if !ok {
		return fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
	if _, err := d.dialector(cfg); err != nil {
		return err
	}
	dbConfig = &cfg
	return nil
}
