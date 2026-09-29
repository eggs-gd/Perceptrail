package model

import (
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//var config gorm.Config = gorm.Config{}

var db *gorm.DB

type proxy struct {
	logger *l.Logger
	db     *gorm.DB
}

func NewProxy(logger *l.Logger) *proxy {
	if db == nil {
		db = initDB(logger)
	}

	return &proxy{logger, db}
}

func initDB(logger *l.Logger) *gorm.DB {
	logger.Info("Database Initiating...")
	// WAL + busy_timeout: importer writes while /items stream reads
	db, err := gorm.Open(sqlite.Open("file:media_library.db?_busy_timeout=5000&_journal_mode=WAL&_fk=1"), &gorm.Config{
		Logger: newLogger(logger),
	})
	if err != nil {
		logger.Fatal("Can't connect to database", l.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("Can't get sql.DB", l.Error(err))
	}
	// One connection: avoids SQLite "database is locked" under concurrent GORM pools.
	// StreamAllItems uses short keyset pages, so the conn is released while the client is written to.
	// Never open a transaction and query db (not tx) inside it — that would deadlock on this single conn.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	err = db.AutoMigrate(
		&dto.ItemDto{},
		&dto.FileDto{},
		// &Tag{},
		// &Album{},
	)
	if err != nil {
		logger.Fatal("Can't do migration", l.Error(err))
	}

	logger.Info("Database migration done!")
	return db
}
