package dto

import "strings"

// AssetKind: what an asset is — the source's word (Apple Photos sets it: its Live
// Photo's original is the video), else by its files' roles: a video original is a
// video, a photo with motion a Live Photo, the rest a photo. A plain folder's video
// with a still of the same name stays a video (it may be a Live Photo's pair or a
// camera's thumbnail: the roles cannot tell). The one rule for the import (it saves
// the kind with the item), the client and the transcoders.
func AssetKind(source string, files []*FileDto) string {
	if source != "" {
		return source
	}
	var video, motion bool
	for _, f := range files {
		switch {
		case f.Role == RoleOriginal && strings.HasPrefix(f.MimeType, "video/"):
			video = true
		case f.Role == RoleMotion:
			motion = true
		}
	}
	switch {
	case video:
		return KindVideo
	case motion:
		return KindLive
	}
	return KindPhoto
}
