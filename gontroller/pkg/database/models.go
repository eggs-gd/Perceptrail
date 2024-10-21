package database

import (
	"time"
)

type Item struct {
	ID        uint      `gorm:"primaryKey"`  // Primary Key
	GUID      string    `gorm:"uniqueIndex"` // GUID унікальний
	HashShort string    `gorm:"index"`       // Хеш короткий
	HashFull  string    `gorm:"index"`       // Хеш повний
	Date      time.Time // Timestamp
	Path      string    // Основний шлях
	SrcSet    []SrcSet  `gorm:"foreignKey:ItemID"`      // SrcSet (прев'ю)
	Exif      Exif      `gorm:"embedded"`               // Вбудована структура Exif
	Tags      []Tag     `gorm:"many2many:item_tags;"`   // Теги
	Albums    []Album   `gorm:"many2many:item_albums;"` // Альбоми
	GeoData   GeoData   `gorm:"embedded"`               // Гео-дані
	Faces     []Face    `gorm:"foreignKey:ItemID"`      // Обличчя
	Objects   []Object  `gorm:"foreignKey:ItemID"`      // Об'єкти
}

type SrcSet struct {
	ID     uint   `gorm:"primaryKey"`
	ItemID uint   // Foreign Key до Item
	Path   string // Шлях до зображення
	Width  int    // Ширина
}

type Exif struct {
	MimeType       string
	ImageWidth     int
	ImageHeight    int
	ExifItemWidth  int
	ExifItemHeight int
	Duration       float32
	AvgBitrate     float32
	VideoCodec     string
	AudioCodec     string
	// Інші поля EXIF можуть бути додані за потреби
	// Наприклад, Exposure, ISO, ShutterSpeed тощо
}

type Tag struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`
}

type Album struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`
}

type GeoData struct {
	Coords  GeoCoords
	Address string
}

type GeoCoords struct {
	Longitude float64
	Latitude  float64
}

type Face struct {
	ID         uint `gorm:"primaryKey"`
	ItemID     uint // Foreign Key to Item
	Label      string
	Descriptor [128]int
	Rect       [4]int
}

type Object struct {
	ID         uint `gorm:"primaryKey"`
	ItemID     uint // Foreign Key to Item
	Label      string
	Descriptor [32]int
	Rect       [4]int
}
