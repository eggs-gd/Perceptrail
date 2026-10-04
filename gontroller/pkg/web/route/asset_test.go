package route

import (
	"testing"

	"perceptrail/gontroller/pkg/library/apple"
	"perceptrail/gontroller/pkg/library/apple/photokit"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

func TestClientAssetByRoles(t *testing.T) {
	item := &dto.ItemDto{Guid: "G", PreviewPath: "/cache/G/embedded.jpg", PreviewMime: "image/jpeg"}
	file := func(id uint, role, mime string, w int) *dto.FileDto {
		f := &dto.FileDto{ID: id, Role: role, Width: w, LinkedTo: "G"}
		f.MimeType = mime
		return f
	}
	a := toClientAsset(item, []*dto.FileDto{
		file(1, dto.RoleOriginal, "image/heic", 4032),
		file(2, dto.RoleStill, "image/jpeg", 2048),
		file(3, dto.RoleStill, "image/jpeg", 360),
		file(4, dto.RoleEdit, "image/jpeg", 4032),
		file(5, dto.RoleMotion, "video/quicktime", 1080),
		file(6, dto.RoleMeta, "application/rdf+xml", 0),
	}, nil)
	if a.Original == nil || a.Original.URL != "/assets/G/1" || a.Original.Mime != "image/heic" {
		t.Errorf("original %+v", a.Original)
	}
	// Smallest first; the extracted embedded preview (unknown size here) is a still too
	if len(a.Stills) != 3 || a.Stills[0].URL != "/assets/G/embedded" || a.Stills[1].W != 360 || a.Stills[2].W != 2048 {
		t.Errorf("stills %+v", a.Stills)
	}
	if len(a.Edit) != 1 || len(a.Motion) != 1 || len(a.Frames) != 0 {
		t.Errorf("edit %+v motion %+v frames %+v", a.Edit, a.Motion, a.Frames)
	}
}

// The kind: the source's word first (an Apple Live Photo's original is its video),
// else by the roles
func TestClientAssetKind(t *testing.T) {
	file := func(role, mime string) *dto.FileDto {
		f := &dto.FileDto{Role: role}
		f.MimeType = mime
		return f
	}
	for _, c := range []struct {
		name, stored string
		files        []*dto.FileDto
		want         string
	}{
		{"photo", "", []*dto.FileDto{file(dto.RoleOriginal, "image/heic")}, dto.KindPhoto},
		{"generic live", "", []*dto.FileDto{file(dto.RoleOriginal, "image/heic"), file(dto.RoleMotion, "video/quicktime")}, dto.KindLive},
		{"video", "", []*dto.FileDto{file(dto.RoleOriginal, "video/mp4"), file(dto.RoleStill, "image/jpeg")}, dto.KindVideo},
		{"apple live", dto.KindLive, []*dto.FileDto{file(dto.RoleOriginal, "video/quicktime"), file(dto.RoleStill, "image/heic")}, dto.KindLive},
	} {
		if got := toClientAsset(&dto.ItemDto{Guid: "G", Kind: c.stored}, c.files, nil).Kind; got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// The client learns which items can ask for more: those from Photos, a hover for
// what moves
func TestOnDemandInAsset(t *testing.T) {
	lib := "/p/Photos Library.photoslibrary/originals/A/A1.heic"
	photos := apple.New("/p", photokit.Library{}, nil, l.NewLogger(l.FatalLevel, &decorators.GontrollerDecorator{}))
	if od := toClientAsset(&dto.ItemDto{Guid: "A1", Kind: dto.KindPhoto, Path: lib}, nil, photos).OnDemand; od == nil ||
		od.Medium != "/items/A1/rendition/medium?v="+contractVersion || od.Hover != "" || od.Original != "/items/A1/rendition/original?v="+contractVersion {
		t.Errorf("photo, original in iCloud: %+v", od)
	}
	here := []*dto.FileDto{{ID: 1, Role: dto.RoleOriginal, LinkedTo: "A2"}}
	here[0].MimeType = "image/heic"
	if od := toClientAsset(&dto.ItemDto{Guid: "A2", Kind: dto.KindPhoto, Path: lib}, here, photos).OnDemand; od == nil || od.Original == "" {
		t.Errorf("photo, original here: %+v, want the original still asked from Photos (it may be edited)", od)
	}
	if od := toClientAsset(&dto.ItemDto{Guid: "V1", Kind: dto.KindVideo, Path: lib}, nil, photos).OnDemand; od == nil ||
		od.Hover != "/items/V1/rendition/hover?v="+contractVersion {
		t.Errorf("video: %+v", od)
	}
	if od := toClientAsset(&dto.ItemDto{Guid: "F1", Path: "/photos/f.jpg"}, nil, nil).OnDemand; od != nil {
		t.Errorf("a folder's photo: %+v, want none", od)
	}
}

// The asset carries the full size of what is seen
func TestClientAssetFull(t *testing.T) {
	item := &dto.ItemDto{Guid: "FULL-1"}
	item.Size.W, item.Size.H = 3024, 4032
	if a := toClientAsset(item, nil, nil); a.Full == nil || a.Full.W != 3024 || a.Full.H != 4032 {
		t.Errorf("full %+v, want the item's size", a.Full)
	}
}
