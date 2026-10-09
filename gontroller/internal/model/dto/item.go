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
	// Values are stored: new states go at the end
	Visible // the cheap stage found something the browser shows (Preview*); transcode later
	Waiting // nothing to show without a transcode: hidden until the expensive stage
)

// Kinds of an asset: the gallery marks moving ones on the tile
const (
	KindPhoto = "photo"
	KindLive  = "live" // a photo with a short video (Live Photo)
	KindVideo = "video"
)

type ItemDto struct {
	gorm.Model
	/* gorm.Model:
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
	*/
	GUID      api.GUID `gorm:"uniqueIndex"`
	HashShort string   `gorm:"index"` // Fast hash based on size, exifdata
	// HashFull  string    `gorm:"index"` // Hash of whole file, make sense only if hashfull is chiper than transode. As an option enable for CPU setups
	MimeType string    `gorm:"index"` //
	State    ItemState `gorm:"index"` // Current state of item

	// What the client is shown until our own previews exist: a file of the group
	// the browser can show (the original, a derivative) or an extracted embedded
	// preview in the cache. "" = nothing (Waiting)
	PreviewPath string
	PreviewMime string
	// PreviewColor: a quiet placeholder while the preview image decodes, as #rrggbb.
	PreviewColor string
	// Hash of the source's own metadata (Apple Photos DB) this item was built from
	MetaHash string
	// Rework: something outside the item's files wants it processed again (a perceptor
	// that has no row for it); publishing clears it
	Rework bool
	// A video's length, seconds; 0: not a video or unknown
	Duration float64
	// What the asset is (Kind*) when the source says it (Apple Photos); "": the
	// client API derives it from the roles of the files
	Kind string

	Date time.Time `gorm:"index"` // CreationDate of asset: the instant (the DB returns it in UTC); the default sheet's order
	// Local zone of the shot, minutes east of UTC: sqlite and Postgres timestamptz
	// drop the zone of Date, so it is kept separately
	DateOffset int
	DateSource string // the tag Date came from; "" = no date
	DateZone   string // how DateOffset was found: tag, gps, coords, file, server (assumed)
	Path       string // Source path

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

// StoredItem: an item as the library keeps it — its row, its files, its renditions;
// what a stream of items carries (a new fact about an item is a new field here, not
// a new parameter everywhere)
type StoredItem struct {
	Item       *ItemDto
	Files      []*FileDto
	Renditions []RenditionDto // smallest first
}

// ItemPublished: an item went through the cheap stage (Visible or Waiting) — the
// expensive stage may have work for it (the model's event, after the commit)
type ItemPublished struct {
	GUID  api.GUID
	State ItemState
}

func (ItemDto) TableName() string {
	return "items"
}

// GetDate returns the date in the local zone of the shot; no date: the zero time
func (i *ItemDto) GetDate() time.Time {
	if i.DateSource == "" {
		return i.Date
	}
	return i.Date.In(time.FixedZone("", i.DateOffset*60))
}
