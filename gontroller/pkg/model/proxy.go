package model

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var config gorm.Config = gorm.Config{}

var db *gorm.DB

type Proxy struct {
	db *gorm.DB
}

func NewProxy() *Proxy {
	if db == nil {
		db = initDB()
	}

	return &Proxy{db}
}

func initDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("media_library.db"), &config)
	if err != nil {
		log.Fatalln("Can't connect to database:", err)
	}

	err = db.AutoMigrate(
		&ItemDto{},
		&FileDto{},
		// &Tag{},
		// &Album{},
	)
	if err != nil {
		log.Fatalln("Can't do migration:", err)
	}

	log.Println("Database migration done!")
	return db
}
