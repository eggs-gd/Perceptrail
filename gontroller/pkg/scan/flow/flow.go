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
	// Set by a grouper that knows the asset (Apple Photos: the asset UUID): the
	// item's GUID, and Files[0] is the main file as the grouper decided — mime
	// does not re-rank. "" (generic): the GUID of the main file, mime ranks.
	Key string
	// What to show first, best first (a keyed group; e.g. the edit before the
	// original); nil: the cheap preview decides by itself
	Show []*dto.FileDto
	// Metadata from the source itself (the Apple Photos DB): wins over the files'
	// EXIF; MetaHash tells the gate it changed while the files did not
	Meta     api.RawExif
	MetaHash string
	// What the asset is (dto.Kind*), when the source says it
	Kind string
	Done *WalkResult
	// With the marker: files the grouper saw but held back (their group did not
	// complete in this walk) — not "gone" for the deletions
	Held []string
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
	Item     *dto.ItemDto
	Exif     []api.RawExif
	Files    []*dto.FileDto
	Kinds    []MediaKind
	Key      string         // FileGroup.Key
	Show     []*dto.FileDto // FileGroup.Show
	Meta     api.RawExif    // FileGroup.Meta: GetExif reads it first
	MetaHash string
	Kind     string // FileGroup.Kind
}

// IsMedia: there is something to show — the main file; in a keyed group any file
// (an Apple asset whose original is broken still has Apple's derivatives)
func (r *RawItem) IsMedia() bool {
	if r.Key != "" {
		return r.HasKind(KindImage) || r.HasKind(KindRaw) || r.HasKind(KindVideo)
	}
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
	if v, ok := r.Meta[key]; ok {
		return string(v)
	}
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
	return r.Item.GetDate()
}

func (r *RawItem) GetDuration() float64 {
	return r.Item.Duration
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

func (r *RawItem) SetDuration(seconds float64) {
	r.Item.Duration = seconds
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
