package model

import (
	"fmt"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"

	"gorm.io/gorm"
)

// ErrNotFound: a lookup found no row (GetFileByPath, GetItemByGuid, …)
var ErrNotFound = gorm.ErrRecordNotFound

// Config: what the model reads of the config — where its data is
type Config interface {
	Database() config.Database
}

// Proxy: the model — the library's data and its rules (ItemsApi, FilesApi, MetaApi
// and the import's rules); one per run, opened in main and passed to who uses it
type Proxy struct {
	logger *l.Logger
	db     *gorm.DB
}

// Open connects to the database of the config and migrates its schema
func Open(cfg Config, logger *l.Logger) (*Proxy, error) {
	d, dialector, err := dialectorOf(cfg.Database())
	if err != nil {
		return nil, err
	}
	logger.Info("Database opening", l.String("driver", cfg.Database().Driver), l.String("name", cfg.Database().Name))
	db, err := gorm.Open(dialector, &gorm.Config{Logger: newLogger(logger)})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := d.tune(db); err != nil {
		return nil, fmt.Errorf("configure: %w", err)
	}
	if err := db.AutoMigrate(&dto.ItemDto{}, &dto.FileDto{}, &dto.MetaDto{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Proxy{logger, db}, nil
}

// Close: the connection closed (the end of the run: its journal merged into the
// file)
func (p *Proxy) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
