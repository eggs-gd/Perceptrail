package dto

import (
	"time"

	"github.com/eggs-gd/perceplib/api"
)

type ItemEntry struct {
	Path     string
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
}

type FileDto struct {
	ID   uint     `gorm:"primaryKey"`
	GUID api.GUID `gorm:"uniqueIndex"`
	// The item the file belongs to: a main file's own GUID, a sidecar's main file's
	// GUID, a keyed group's key; "" — not decided yet; api.NilGUID — no item
	// (ignored: not media, broken)
	LinkedTo api.GUID `gorm:"index"`

	// What the file is to its asset (Role*): set by the source's grouper when it
	// knows (Apple Photos), otherwise by the mime step
	Role string
	// Pixel size (images: read from the file's header; the main file: from its
	// metadata); 0 = unknown
	Width  int
	Height int
	// A video's codec (exiftool CompressorID: avc1, hvc1, …) when known: lets the
	// client offer <source type="video/mp4; codecs=…"> and the browser choose
	Codec string

	ItemEntry

	// Changed: new, its stat or its role changed, and its group not decided since
	// (stored: a pass that fails before identify decides — a library's DB not
	// loaded, an asset not complete, an error — leaves it set for the next one);
	// set by the walk and a grouper, cleared by identify's validate
	Changed bool `json:"-"`
}

// WalkedFile: a file as the walk gives it to the groupers — its row, and whether
// the walk found it missing (a complete walk did not see it). A fact of this pass,
// not stored.
type WalkedFile struct {
	*FileDto
	Missing bool
}

// Roles of the files of an asset: the client picks by them (a still for the tile,
// motion on hover, the biggest still in the viewer, the original on demand)
const (
	RoleOriginal = "original" // the source (may not be viewable: HEIC, RAW, HEVC)
	RoleEdit     = "edit"     // the user's edit, full size or a preview of it
	RoleStill    = "still"    // a viewable image of the asset, any size
	RoleMotion   = "motion"   // a video of the asset (a Live Photo's video)
	RoleFrames   = "frames"   // one frame of a sequence (a flip-book on hover)
	RoleMeta     = "meta"     // metadata only (.xmp, .aae): not shown
)

// SetRole: the source's grouper says what the file is to its asset; a new role is
// new work
func (f *FileDto) SetRole(role string) {
	if f.Role != role {
		f.Role = role
		f.Changed = true
	}
}

func (FileDto) TableName() string {
	return "files"
}

func (f *FileDto) LinkTo(mainFile *FileDto) bool {
	return f.LinkToItem(mainFile.GUID)
}

// LinkToItem links the file to an item by its GUID (a keyed group: the asset's key)
func (f *FileDto) LinkToItem(guid api.GUID) bool {
	if f.LinkedTo != guid {
		f.LinkedTo = guid
		return true
	}
	return false
}

func (f *FileDto) SetIgnored() {
	f.LinkedTo = api.NilGUID
}

func (f *FileDto) IsIgnored() bool {
	return f.LinkedTo == api.NilGUID
}
