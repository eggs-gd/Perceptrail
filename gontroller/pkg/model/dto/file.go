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

	ItemEntry
}

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
