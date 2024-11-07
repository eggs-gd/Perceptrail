package model

import (
	"gontroller/pkg/model/dto"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var config gorm.Config = gorm.Config{}

var db *gorm.DB

type Proxy struct {
	logger *zap.Logger
	db     *gorm.DB
}

func NewProxy(logger *zap.Logger) *Proxy {
	if db == nil {
		db = initDB(logger)
	}

	return &Proxy{logger, db}
}

func initDB(logger *zap.Logger) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("media_library.db"), &config)
	if err != nil {
		logger.Fatal("Can't connect to database", zap.Error(err))
	}

	err = db.AutoMigrate(
		&dto.ItemDto{},
		&dto.FileDto{},
		// &Tag{},
		// &Album{},
	)
	if err != nil {
		logger.Fatal("Can't do migration", zap.Error(err))
	}

	logger.Info("Database migration done!")
	return db
}
