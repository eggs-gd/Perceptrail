package database

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var config gorm.Config = gorm.Config{}

var Db *gorm.DB

func InitDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("media_library.db"), &config)
	if err != nil {
		log.Fatalln("Can't connect to database:", err)
	}

	err = db.AutoMigrate(
		&Item{},
		&SrcSet{},
		&Exif{},
		&Tag{},
		&Album{},
		&GeoData{},
		&Face{},
		&Object{},
	)
	if err != nil {
		log.Fatalln("Can't do migration:", err)
	}

	log.Println("Database migration done!")
	return db
}

func init() {
	Db = InitDB()
}
