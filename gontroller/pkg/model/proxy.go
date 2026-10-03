package model

import (
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"

	"gorm.io/gorm"
)

//var config gorm.Config = gorm.Config{}

var db *gorm.DB

// ErrNotFound: a lookup found no row (GetFileByPath, GetItemByGuid, …)
var ErrNotFound = gorm.ErrRecordNotFound

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
	if dbConfig == nil {
		panic("model: Configure must be called before NewProxy")
	}
	cfg := *dbConfig
	d := drivers[cfg.Driver]
	logger.Info("Database Initiating...", l.String("driver", cfg.Driver), l.String("name", cfg.Name))

	dialector, err := d.dialector(cfg)
	if err != nil {
		logger.Fatal("Can't open database", l.Error(err))
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: newLogger(logger),
	})
	if err != nil {
		logger.Fatal("Can't connect to database", l.Error(err))
	}
	if err := d.tune(db); err != nil {
		logger.Fatal("Can't configure database", l.Error(err))
	}

	err = db.AutoMigrate(
		&dto.ItemDto{},
		&dto.FileDto{},
		&dto.MetaDto{},
		// &Tag{},
		// &Album{},
	)
	if err != nil {
		logger.Fatal("Can't do migration", l.Error(err))
	}

	logger.Info("Database migration done!")
	return db
}
