package transcode

import (
	"strings"

	"perceptrail/gontroller/pkg/model/dto"
)

// Item: what the transcoders work on — an item and its files with their roles, as
// the DB keeps them (the expensive stage is fed from the DB, not by the import)
type Item struct {
	Item  *dto.ItemDto
	Files []*dto.FileDto
}

// kind: what the asset is — the source's word (Apple Photos), else by the roles:
// a video original with a still is a Live Photo, a video original a video, the
// rest a photo
func (it *Item) kind() string {
	if it.Item != nil && it.Item.Kind != "" {
		return it.Item.Kind
	}
	var video, still bool
	for _, f := range it.Files {
		switch {
		case f.Role == dto.RoleOriginal && strings.HasPrefix(f.MimeType, "video/"):
			video = true
		case f.Role == dto.RoleStill && strings.HasPrefix(f.MimeType, "image/"):
			still = true
		}
	}
	switch {
	case video && still:
		return dto.KindLive
	case video:
		return dto.KindVideo
	}
	return dto.KindPhoto
}
