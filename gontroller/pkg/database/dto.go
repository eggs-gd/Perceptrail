package model

import (
	t "gontroller/pkg/_t"
	"time"

	"gorm.io/gorm"
)

type ItemDo struct {
	gorm.Model
	/* gorm.Model is:
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
	*/
	GUID      string `gorm:"uniqueIndex"`
	HashShort string `gorm:"index"` // Fast hash based on size, exifdata
	HashFull  string `gorm:"index"` // Hash of whole file

	Date time.Time // CreationDate of asset
	Path string    // Source path

	Size  t.Size `gorm:"embedded;embeddedPrefix:size_"`
	Ratio t.Size `gorm:"embedded;embeddedPrefix:ratio_"`

	// Tags   []Tag   `gorm:"many2many:item_tags;"`
	// Albums []Album `gorm:"many2many:item_albums;"`

	//----------------
	// SrcSet []SrcSet `gorm:"foreignKey:ItemID"`             // Will be created dynamically with given presets, looks like not needed
	// Exif   Exif     `gorm:"embedded;embeddedPrefix:exif_"` // Whole exif data from asset, looks like not needed

	// Geo
	// GeoData GeoData `gorm:"embedded;embeddedPrefix:geo_"`

	// ML
	// Faces   []Face   `gorm:"foreignKey:ItemID"`
	// Objects []Object `gorm:"foreignKey:ItemID"`
}

// type SrcSet struct {
// 	ID     uint   `gorm:"primaryKey"`
// 	ItemID uint   // Foreign Key до Item
// 	Path   string // Шлях до зображення
// 	Width  int    // Ширина
// }

// type Exif struct {
// 	MimeType       string
// 	ImageWidth     int
// 	ImageHeight    int
// 	ExifItemWidth  int
// 	ExifItemHeight int
// 	Duration       float32
// 	AvgBitrate     float32
// 	VideoCodec     string
// 	AudioCodec     string
// 	// Other fields
// }

// type Tag struct {
// 	ID        uint   `gorm:"primaryKey"`
// 	Name      string `gorm:"uniqueIndex"`
// 	CreatedAt time.Time
// 	UpdatedAt time.Time
// }

// type Album struct {
// 	ID        uint   `gorm:"primaryKey"`
// 	Name      string `gorm:"uniqueIndex"`
// 	CreatedAt time.Time
// 	UpdatedAt time.Time
// }

// type GeoData struct {
// 	Longitude float64
// 	Latitude  float64
// 	Address   string
// }

// type Face struct {
// 	ID         uint `gorm:"primaryKey"`
// 	ItemID     uint // Foreign Key to Item
// 	Label      string
// 	Descriptor [128]int
// 	Rect       [4]int
// }

// type Object struct {
// 	ID         uint `gorm:"primaryKey"`
// 	ItemID     uint // Foreign Key to Item
// 	Label      string
// 	Descriptor [32]int
// 	Rect       [4]int
// }
