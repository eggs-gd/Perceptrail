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

	return &Proxy{
		db: db,
	}
}

func (p *Proxy) GetItemPathHash(path string, hashShort string) (ItemDto, error) {
	var item ItemDto

	// Check if the item exists in the database
	err := p.db.Where("path = ?", path).Or("hash_short = ?", hashShort).First(&item).Error
	return item, err
}

func (p *Proxy) UpdateItem(item ItemDto) error {
	return p.db.Save(&item).Error
}

func (p *Proxy) CreateItem(item ItemDto) error {
	//todo update guid
	return db.Create(&item).Error
}

func initDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("media_library.db"), &config)
	if err != nil {
		log.Fatalln("Can't connect to database:", err)
	}

	err = db.AutoMigrate(
		&ItemDto{},
		// &Tag{},
		// &Album{},
	)
	if err != nil {
		log.Fatalln("Can't do migration:", err)
	}

	log.Println("Database migration done!")
	return db
}
