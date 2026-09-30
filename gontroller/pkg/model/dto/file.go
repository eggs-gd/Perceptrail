package dto

import (
	"time"
)

type ItemEntry struct {
	Path     string
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
}

type FileDto struct {
	ID   uint   `gorm:"primaryKey"`
	GUID string `gorm:"uniqueIndex"`
	// - Main file contans its own QUID
	// - Sidecar contains Guid of main file
	// - And contains "-" if ignored/unwanted
	LinkedTo string `gorm:"index"`
	// Time of last walker run used as key to find deleted files
	// Finalization step should check if there is some entries with CheckTime different from current
	// It means that in previous runs this files was present but now deleted
	CheckTime time.Time `gorm:"index"`

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

func (FileDto) TableName() string {
	return "files"
}

func (f *FileDto) LinkTo(mainFile *FileDto) bool {
	return f.LinkToItem(mainFile.GUID)
}

// LinkToItem links the file to an item by its GUID (a keyed group: the asset's key)
func (f *FileDto) LinkToItem(guid string) bool {
	if f.LinkedTo != guid {
		f.LinkedTo = guid
		return true
	}
	return false
}

func (f *FileDto) SetIgnored() {
	f.LinkedTo = "-"
}

func (f *FileDto) IsIgnored() bool {
	return f.LinkedTo == "-"
}
