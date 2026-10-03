// Package flow: the item transcode still reads (RawItem, MediaKind), left until
// transcode gets an input of its own; the import no longer uses it — identify's
// working item is private there, it yields identify.Item.
package flow

import (
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

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
	// Embedded: a preview extracted from the main file (identify), when the group has
	// nothing the browser shows — the cheap preview's last resort
	Embedded string
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

// The perceptors' values ride on the item (committed by the closer)
func (r *RawItem) StoreValues(store string) (api.Values, bool) { return r.Item.StoreValues(store) }
func (r *RawItem) SetStoreValues(store string, v api.Values)   { r.Item.SetStoreValues(store, v) }

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
)
