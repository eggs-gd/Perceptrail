// Package flow holds what flows between the steps of the import chain
// (_sb/puml/Import chain.puml): the chain and its sub-packages (groups/…,
// transcode/…) share these types.
package flow

import (
	"time"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
)

// FileEvent: fswalker -> source switch -> groupers. One found file (Entry: path
// and stat), or the end-of-walk marker (Done).
type FileEvent struct {
	Entry dto.ItemEntry
	Done  *WalkResult
}

// FileGroup is one whole asset: all its files (a main file and its sidecars,
// derivatives), or the end-of-walk marker. Groupers build it (files not stored
// yet: only the stat is set), the files gate stores it (rows of the files table,
// with GUIDs), exif turns it into a RawItem. Done is set on a grouper's marker,
// which may come together with its last group.
type FileGroup struct {
	Files []*dto.FileDto
	Done  *WalkResult
}

// WalkResult describes a finished walk; it rides in the end-of-walk marker.
// Deletions may be derived from it only if the walk was complete: a cancelled walk
// or an unreadable root says nothing about which files are gone.
type WalkResult struct {
	Root    string
	Started time.Time
	// The walk reached the end
	Complete bool
	// Files seen (before grouping and filtering)
	Files int
	// Directories that could not be read: their files are not "deleted"
	Unreadable []string
}

// MediaKind is the role a file can play in a group (set by the mime step)
type MediaKind string

const (
	KindImage   MediaKind = "image"
	KindRaw     MediaKind = "raw"
	KindVideo   MediaKind = "video"
	KindSidecar MediaKind = "sidecar"
	KindOther   MediaKind = "other"
)

// RawItem is an item on its way from exif to the closer: its group of files.
// Files, Exif and Kinds are aligned (a nil Exif: exiftool returned nothing).
// exif fills Files and Exif, mime fills Kinds and puts the main file first, the
// validator sets Item.
type RawItem struct {
	Item  *dto.ItemDto
	Exif  []api.RawExif
	Files []*dto.FileDto
	Kinds []MediaKind
}

// IsMedia: the main file is something to show
func (r *RawItem) IsMedia() bool {
	switch r.Kinds[0] {
	case KindImage, KindRaw, KindVideo:
		return true
	}
	return false
}

// HasKind: some file of the group is of kind k
func (r *RawItem) HasKind(k MediaKind) bool {
	for _, kind := range r.Kinds {
		if kind == k {
			return true
		}
	}
	return false
}

// ExifProvider implementation
func (r *RawItem) GetExif(key string) string {
	for _, e := range r.Exif {
		if bytes, ok := e[key]; ok {
			return string(bytes)
		}
	}
	return ""
}

// ItemDataProvider implementation
func (r *RawItem) GetGuid() string {
	return r.Item.Guid
}

// GetDate returns the date in the local zone of the shot
func (r *RawItem) GetDate() time.Time {
	if r.Item.DateSource == "" {
		return r.Item.Date
	}
	return r.Item.Date.In(time.FixedZone("", r.Item.DateOffset*60))
}

func (r *RawItem) GetSize() api.Size {
	return r.Item.Size
}

func (r *RawItem) GetRatio() api.Size {
	return r.Item.Ratio
}

// ItemDataEditor implementation
func (r *RawItem) SetDate(date time.Time) {
	r.SetDateInfo(date, "plugin", "tag")
}

func (r *RawItem) SetDateInfo(date time.Time, source, zone string) {
	_, offset := date.Zone()
	r.Item.Date = date.UTC()
	r.Item.DateOffset = offset / 60
	r.Item.DateSource = source
	r.Item.DateZone = zone
}

func (r *RawItem) SetSize(size api.Size) {
	r.Item.Size = size
}

func (r *RawItem) SetRatio(ratio api.Size) {
	r.Item.Ratio = ratio
}

// Verify interface implementations
var (
	_ api.ExifProvider     = (*RawItem)(nil)
	_ api.ItemDataProvider = (*RawItem)(nil)
	_ api.ItemDataEditor   = (*RawItem)(nil)
	_ api.RawItemR         = (*RawItem)(nil)
	_ exif_core.RawItemRW  = (*RawItem)(nil)
)
