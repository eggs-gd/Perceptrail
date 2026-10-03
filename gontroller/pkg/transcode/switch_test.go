package transcode

import (
	"testing"

	"perceptrail/gontroller/pkg/model/dto"
)

func TestTranscodeSwitch(t *testing.T) {
	file := func(role, mime string) *dto.FileDto {
		return &dto.FileDto{Role: role, ItemEntry: dto.ItemEntry{MimeType: mime}}
	}
	route := func(kind string, files ...*dto.FileDto) int {
		b, _ := Switch{}.Route(&Item{Item: &dto.ItemDto{Kind: kind}, Files: files})
		return b
	}
	cases := []struct {
		name  string
		kind  string
		files []*dto.FileDto
		want  int
	}{
		{"a plain folder's video with a still: a video", "", []*dto.FileDto{file(dto.RoleOriginal, "video/quicktime"), file(dto.RoleStill, "image/heic")}, BranchVideo},
		{"a photo with motion", "", []*dto.FileDto{file(dto.RoleOriginal, "image/heic"), file(dto.RoleMotion, "video/quicktime")}, BranchLivePhoto},
		{"video", "", []*dto.FileDto{file(dto.RoleOriginal, "video/mp4")}, BranchVideo},
		{"RAW with its JPEG", "", []*dto.FileDto{file(dto.RoleOriginal, "image/x-nikon-nef"), file(dto.RoleStill, "image/jpeg")}, BranchPhoto},
		{"photo with a sidecar", "", []*dto.FileDto{file(dto.RoleOriginal, "image/jpeg"), file(dto.RoleMeta, "application/rdf+xml")}, BranchPhoto},
		{"the source says: a video (its poster is a still)", dto.KindVideo, []*dto.FileDto{file(dto.RoleOriginal, "video/quicktime"), file(dto.RoleStill, "image/jpeg")}, BranchVideo},
		{"the source says: a Live Photo", dto.KindLive, []*dto.FileDto{file(dto.RoleOriginal, "image/heic"), file(dto.RoleMotion, "video/quicktime")}, BranchLivePhoto},
	}
	for _, c := range cases {
		if got := route(c.kind, c.files...); got != c.want {
			t.Errorf("%s: branch %d, want %d", c.name, got, c.want)
		}
	}
}
