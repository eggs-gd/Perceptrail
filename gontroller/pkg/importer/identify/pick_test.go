package identify

import (
	"errors"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

type pf struct {
	name string
	mime string
	kind flow.MediaKind
	exif api.RawExif
}

func pick(t *testing.T, files ...pf) (string, string, []string) {
	t.Helper()
	var extracted []string
	e := &Embedded{
		logger: l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}),
		Extract: func(tag, src, guid string) (string, error) {
			extracted = append(extracted, tag)
			if tag == "JpgFromRaw" {
				return "", errors.New("broken") // falls through to the next tag
			}
			return "/cache/" + guid + "/embedded.jpg", nil
		},
	}
	it := &flow.RawItem{Item: &dto.ItemDto{Guid: "g"}}
	for _, f := range files {
		it.Files = append(it.Files, &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/" + f.name, Name: f.name, MimeType: f.mime}})
		it.Kinds = append(it.Kinds, f.kind)
		it.Exif = append(it.Exif, f.exif)
	}
	it, _ = e.Decorate(it)
	it, _ = Pick{}.Decorate(it)
	return it.Item.PreviewPath, it.Item.PreviewMime, extracted
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
			[]pf{{"a.jpg", "image/jpeg", flow.KindImage, size("4000", "3000")}},
			"/lib/a.jpg", "image/jpeg"},
		{"RAW: the biggest viewable derivative",
			[]pf{{"d.nef", "image/x-nikon-nef", flow.KindRaw, api.RawExif{"PreviewImage": []byte("x")}},
				{"d.small.jpg", "image/jpeg", flow.KindImage, size("640", "480")},
				{"d.jpg", "image/jpeg", flow.KindImage, size("6000", "4000")},
				{"d.xmp", "application/rdf+xml", flow.KindSidecar, nil}},
			"/lib/d.jpg", "image/jpeg"},
		{"RAW alone: its embedded preview (the next tag when one fails)",
			[]pf{{"d.nef", "image/x-nikon-nef", flow.KindRaw, api.RawExif{"JpgFromRaw": []byte("x"), "PreviewImage": []byte("x")}}},
			"/cache/g/embedded.jpg", "image/jpeg"},
		{"HEIC without anything viewable: waits",
			[]pf{{"i.heic", "image/heic", flow.KindImage, size("4032", "3024")}},
			"", ""},
		{"H.264 video shows itself",
			[]pf{{"v.mov", "video/quicktime", flow.KindVideo, api.RawExif{"CompressorID": []byte("avc1")}}},
			"/lib/v.mov", "video/quicktime"},
		{"HEVC Live Photo with a HEIC photo: waits",
			[]pf{{"l.mov", "video/quicktime", flow.KindVideo, api.RawExif{"CompressorID": []byte("hvc1")}},
				{"l.heic", "image/heic", flow.KindImage, nil}},
			"", ""},
		{"HEVC video with a viewable photo: the photo",
			[]pf{{"l.mov", "video/quicktime", flow.KindVideo, api.RawExif{"CompressorID": []byte("hvc1")}},
				{"l.jpg", "image/jpeg", flow.KindImage, nil}},
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

	if _, _, tags := pick(t, pf{"d.nef", "image/x-nikon-nef", flow.KindRaw, api.RawExif{"JpgFromRaw": []byte("x"), "PreviewImage": []byte("x")}}); len(tags) != 2 || tags[0] != "JpgFromRaw" {
		t.Errorf("embedded previews tried %v, want JpgFromRaw first", tags)
	}
}

// An embedded preview gets the RAW's orientation (with a real exiftool; a JPEG with
// an embedded thumbnail stands in for the RAW)
func TestEmbeddedPreviewOrientation(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not installed")
	}
	dir := t.TempDir()
	writeJPEG := func(name string, w, h int) string {
		t.Helper()
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := jpeg.Encode(f, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
			t.Fatal(err)
		}
		return p
	}
	src, thumb := writeJPEG("raw.jpg", 64, 48), writeJPEG("thumb.jpg", 16, 12)
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	pool := newExiftoolPool(1, logger)
	defer pool.Close()
	if _, err := pool.Command("-overwrite_original", "-ThumbnailImage<="+thumb, "-Orientation#=6", src); err != nil {
		t.Fatal(err)
	}

	e := &Embedded{logger: logger, pool: pool, dir: filepath.Join(dir, "previews")}
	dst, err := e.exiftoolExtract("ThumbnailImage", src, "g")
	if err != nil {
		t.Fatal(err)
	}
	out, err := pool.Command("-s3", "-n", "-Orientation", dst)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "6" {
		t.Errorf("preview orientation %q, want 6 (the RAW's)", got)
	}
}
