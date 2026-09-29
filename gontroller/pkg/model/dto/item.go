package dto

import (
	"time"

	"gorm.io/gorm"

	"github.com/eggs-gd/perceplib/api"
)

type ItemState int

const (
	New        ItemState = iota // Just have source path and not veryfied size/date from raw source (View can show preloaders)
	Dirty                       // Something changed and have to be rechecked (the walker re-emits it)
	Processing                  // transcoding in progress but real size is veryfied
	Ready                       // all done
	Deleted                     // Deleted
)

type ItemDto struct {
	gorm.Model
	/* gorm.Model:
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
	*/
	Guid      string `gorm:"uniqueIndex"`
	HashShort string `gorm:"index"` // Fast hash based on size, exifdata
	// HashFull  string    `gorm:"index"` // Hash of whole file, make sense only if hashfull is chiper than transode. As an option enable for CPU setups
	MimeType string    `gorm:"index"` //
	State    ItemState `gorm:"index"` // Current state of item

	Date time.Time // CreationDate of asset
	Path string    // Source path

	Size  api.Size `gorm:"embedded;embeddedPrefix:size_"`
	Ratio api.Size `gorm:"embedded;embeddedPrefix:ratio_"`

	// Tags   []Tag   `gorm:"many2many:item_tags;"`
	// Albums []Album `gorm:"many2many:item_albums;"`

	// Geo
	// GeoData GeoData `gorm:"embedded;embeddedPrefix:geo_"`

	// ML
	// Faces   []Face   `gorm:"foreignKey:ItemID"`
	// Objects []Object `gorm:"foreignKey:ItemID"`
}

func (ItemDto) TableName() string {
	return "items"
}

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
