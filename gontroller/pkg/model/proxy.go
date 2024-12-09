package model

import (
	"perceptrail/gontroller/pkg/model/dto"

	l "perceptrail/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var config gorm.Config = gorm.Config{}

var db *gorm.DB

type Proxy struct {
	logger *l.Logger
	db     *gorm.DB
}

func NewProxy(logger *l.Logger) *Proxy {
	if db == nil {
		db = initDB(logger)
	}

	return &Proxy{logger, db}
}

func initDB(logger *l.Logger) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("media_library.db"), &config)
	if err != nil {
		logger.Fatal("Can't connect to database", l.Error(err))
	}

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
