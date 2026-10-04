package identify

import (
	"errors"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

type pf struct {
	name string
	mime string
	kind mediaKind
	exif api.RawExif
}

func pick(t *testing.T, files ...pf) (string, string, []string) {
	t.Helper()
	tool := &fakeTool{}
	it := &draft{Item: &dto.ItemDto{Guid: "g"}}
	for _, f := range files {
		it.Files = append(it.Files, &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/" + f.name, Name: f.name, MimeType: f.mime}})
		it.Kinds = append(it.Kinds, f.kind)
		it.Exif = append(it.Exif, f.exif)
	}
	path, mime := preview(it, tool, "/cache/previews", l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	return path, mime, tool.extracted
}

// fakeTool: an Exiftool that writes nothing — JpgFromRaw fails (the next tag is
// tried), every other tag "is written"
type fakeTool struct{ extracted []string }

func (f *fakeTool) Read(paths, tags []string) ([]api.RawExif, error) { return nil, nil }
func (f *fakeTool) Close()                                           {}

func (f *fakeTool) Extract(tag, src, dst string) error {
	f.extracted = append(f.extracted, tag)
	if tag == "JpgFromRaw" {
		return errors.New("broken")
	}
	return nil
}

func TestCheapPreviewPick(t *testing.T) {
	size := func(w, h string) api.RawExif { return api.RawExif{"ImageWidth": []byte(w), "ImageHeight": []byte(h)} }

	cases := []struct {
		name  string
		files []pf
		path  string
		mime  string
	}{
		{"a JPEG shows itself",
			[]pf{{"a.jpg", "image/jpeg", kindImage, size("4000", "3000")}},
			"/lib/a.jpg", "image/jpeg"},
		{"RAW: the biggest viewable derivative",
			[]pf{{"d.nef", "image/x-nikon-nef", kindRaw, api.RawExif{"PreviewImage": []byte("x")}},
				{"d.small.jpg", "image/jpeg", kindImage, size("640", "480")},
				{"d.jpg", "image/jpeg", kindImage, size("6000", "4000")},
				{"d.xmp", "application/rdf+xml", kindSidecar, nil}},
			"/lib/d.jpg", "image/jpeg"},
		{"RAW alone: its embedded preview (the next tag when one fails)",
			[]pf{{"d.nef", "image/x-nikon-nef", kindRaw, api.RawExif{"JpgFromRaw": []byte("x"), "PreviewImage": []byte("x")}}},
			"/cache/previews/g/embedded.jpg", "image/jpeg"},
		{"HEIC without anything viewable: waits",
			[]pf{{"i.heic", "image/heic", kindImage, size("4032", "3024")}},
			"", ""},
		{"H.264 video shows itself",
			[]pf{{"v.mov", "video/quicktime", kindVideo, api.RawExif{"CompressorID": []byte("avc1")}}},
			"/lib/v.mov", "video/quicktime"},
		{"HEVC Live Photo with a HEIC photo: waits",
			[]pf{{"l.mov", "video/quicktime", kindVideo, api.RawExif{"CompressorID": []byte("hvc1")}},
				{"l.heic", "image/heic", kindImage, nil}},
			"", ""},
		{"HEVC video with a viewable photo: the photo",
			[]pf{{"l.mov", "video/quicktime", kindVideo, api.RawExif{"CompressorID": []byte("hvc1")}},
				{"l.jpg", "image/jpeg", kindImage, nil}},
			"/lib/l.jpg", "image/jpeg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path, mime, _ := pick(t, c.files...)
			if path != c.path || mime != c.mime {
				t.Errorf("got %q %q, want %q %q", path, mime, c.path, c.mime)
			}
		})
	}

	if _, _, tags := pick(t, pf{"d.nef", "image/x-nikon-nef", kindRaw, api.RawExif{"JpgFromRaw": []byte("x"), "PreviewImage": []byte("x")}}); len(tags) != 2 || tags[0] != "JpgFromRaw" {
		t.Errorf("embedded previews tried %v, want JpgFromRaw first", tags)
	}
}
