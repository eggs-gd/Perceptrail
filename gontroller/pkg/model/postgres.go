package model

import (
	"errors"

	"gorm.io/gorm"
)

// postgresDriver is a placeholder: the config and the driver switch already
// accept it. Implementing it = gorm.io/driver/postgres with a DSN from
// Host/Port/Username/Password (or Token)/Name, and the default pool.
var postgresDriver = driver{
	dialector: func(cfg DBConfig) (gorm.Dialector, error) {
		return nil, errors.New("database driver postgres: not implemented yet")
	},
	tune: func(db *gorm.DB) error { return nil },
}
