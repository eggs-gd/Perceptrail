package scan

import (
	"time"

	"perceptrail/api"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins/exif_core"
)

type RawItem struct {
	Item *dto.ItemDto
	Exif []api.RawExif
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

func (r *RawItem) GetDate() time.Time {
	return r.Item.Date
}

func (r *RawItem) GetSize() api.Size {
	return r.Item.Size
}

func (r *RawItem) GetRatio() api.Size {
	return r.Item.Ratio
}

// ItemDataEditor implementation
func (r *RawItem) SetDate(date time.Time) {
	r.Item.Date = date
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
