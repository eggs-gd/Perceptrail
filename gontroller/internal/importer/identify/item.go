package identify

import (
	"time"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// Item: what identify yields — the item known (identity, kind, what to show now)
// and its metadata package. The perceptors read it (api.RawItemR) and the core ones
// write into it (builtin.Item); commit publishes Item.
type Item struct {
	Item   *dto.ItemDto
	meta   api.RawExif
	values map[string]api.Values // the perceptors' values (commit keeps them)
}

// mediaKind: what a file is to its group (set by classify)
type mediaKind string

const (
	kindImage   mediaKind = "image"
	kindRaw     mediaKind = "raw"
	kindVideo   mediaKind = "video"
	kindSidecar mediaKind = "sidecar"
	kindOther   mediaKind = "other"
)

// draft: the asset while identify works on it — private to the stage; only Item
// leaves it. Files, Exif and Kinds are aligned (a nil Exif: exiftool returned
// nothing). read fills Exif, Kinds (the main file first), Merged and Hash; validate
// Item; show the item's sizes and preview.
type draft struct {
	dto.Asset
	Item  *dto.ItemDto
	Exif  []api.RawExif
	Kinds []mediaKind
	// Merged: the asset's metadata package (merge) — what the perceptors read
	Merged api.RawExif
	// Hash: the main file's fingerprint, its identity across paths
	Hash string
}

// GetExif: a tag of the asset's metadata package (merge: the source, the
// metadata sidecars, the main file, the derivatives — the first that has it)
func (it *Item) GetExif(key string) string { return string(it.meta[key]) }

func (it *Item) GetGuid() string { return it.Item.Guid }

// GetDate returns the date in the local zone of the shot
func (it *Item) GetDate() time.Time { return it.Item.GetDate() }

func (it *Item) GetDuration() float64 { return it.Item.Duration }

func (it *Item) GetSize() api.Size { return it.Item.Size }

func (it *Item) GetRatio() api.Size { return it.Item.Ratio }

func (it *Item) SetDuration(s float64) { it.Item.Duration = s }

func (it *Item) SetSize(size api.Size) { it.Item.Size = size }

func (it *Item) SetRatio(ratio api.Size) { it.Item.Ratio = ratio }

// The perceptors' values ride on the item (commit keeps them)
func (it *Item) StoreValues(store string) (api.Values, bool) {
	v, ok := it.values[store]
	return v, ok
}

func (it *Item) SetStoreValues(store string, v api.Values) {
	if it.values == nil {
		it.values = map[string]api.Values{}
	}
	it.values[store] = v
}

func (it *Item) SetDateInfo(date time.Time, source, zone string) {
	_, offset := date.Zone()
	it.Item.Date = date.UTC()
	it.Item.DateOffset = offset / 60
	it.Item.DateSource = source
	it.Item.DateZone = zone
}

// isMedia: there is something to show — the main file; in a keyed group any file
// (an Apple asset whose original is broken still has Apple's derivatives)
func (d *draft) isMedia() bool {
	if d.Key != "" {
		return d.hasKind(kindImage) || d.hasKind(kindRaw) || d.hasKind(kindVideo)
	}
	switch d.Kinds[0] {
	case kindImage, kindRaw, kindVideo:
		return true
	}
	return false
}

// hasKind: some file of the group is of kind k
func (d *draft) hasKind(k mediaKind) bool {
	for _, kind := range d.Kinds {
		if kind == k {
			return true
		}
	}
	return false
}

// yield: what leaves the stage — the item and its metadata package; the rest of
// the draft stays here
func yield(d *draft) *Item { return &Item{Item: d.Item, meta: d.Merged} }
