package render

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// The Orientation read from a JPEG's EXIF in either byte order; none without one.
// A portrait JPEG asking for a quarter turn is stale, a landscape one is not.
func TestJPEGOrientation(t *testing.T) {
	for _, c := range []struct {
		name        string
		order       binary.ByteOrder
		orientation uint16
		w, h        int
		stale       bool
	}{
		{"II portrait 6", binary.LittleEndian, 6, 30, 40, true},
		{"MM portrait 8", binary.BigEndian, 8, 30, 40, true},
		{"II landscape 6", binary.LittleEndian, 6, 40, 30, false},
		{"MM portrait 3", binary.BigEndian, 3, 30, 40, false},
		{"none", nil, 0, 30, 40, false},
	} {
		data := withExif(t, c.w, c.h, c.order, c.orientation)
		if got := jpegOrientation(bytes.NewReader(data)); got != int(c.orientation) {
			t.Errorf("%s: orientation %d, want %d", c.name, got, c.orientation)
		}
		path := filepath.Join(t.TempDir(), "p.jpg")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := staleOrientation(path); got != c.stale {
			t.Errorf("%s: stale %v, want %v", c.name, got, c.stale)
		}
	}
}

// Not a JPEG, cut short, a broken EXIF: no orientation, nothing stale
func TestJPEGOrientationBroken(t *testing.T) {
	good := withExif(t, 30, 40, binary.LittleEndian, 6)
	exifAt := bytes.Index(good, []byte("Exif"))
	badOrder := bytes.Clone(good)
	copy(badOrder[exifAt+6:], "XX")
	for name, data := range map[string][]byte{
		"empty":         nil,
		"not a JPEG":    []byte("GIF89a"),
		"cut in EXIF":   good[:exifAt+10],
		"cut at a head": good[:3],
		"byte order":    badOrder,
	} {
		if got := jpegOrientation(bytes.NewReader(data)); got != 0 {
			t.Errorf("%s: orientation %d, want 0", name, got)
		}
	}
	if got := tiffOrientation([]byte("II\x2a\x00\xff\x00\x00\x00")); got != 0 {
		t.Errorf("IFD past the end: %d", got)
	}
	if got := tiffOrientation([]byte("II\x2a\x00\x08\x00\x00\x00\x05\x00")); got != 0 {
		t.Errorf("entries past the end: %d", got)
	}
	if staleOrientation(filepath.Join(t.TempDir(), "missing.jpg")) {
		t.Error("a missing file is stale")
	}
}

// withExif: a w × h JPEG with an Exif APP1 holding only Orientation (order nil: no
// EXIF at all)
func withExif(t *testing.T, w, h int, order binary.ByteOrder, orientation uint16) []byte {
	t.Helper()
	var img bytes.Buffer
	if err := jpeg.Encode(&img, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	if order == nil {
		return img.Bytes()
	}
	var tiff bytes.Buffer
	if order == binary.LittleEndian {
		tiff.WriteString("II")
	} else {
		tiff.WriteString("MM")
	}
	put := func(v any) { _ = binary.Write(&tiff, order, v) }
	put(uint16(42))
	put(uint32(8))
	put(uint16(1))              // one entry
	put(uint16(orientationTag)) // tag
	put(uint16(3))              // SHORT
	put(uint32(1))              // count
	put(orientation)            // value, left-justified
	put(uint16(0))
	put(uint32(0)) // no next IFD
	app1 := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	var out bytes.Buffer
	out.Write(img.Bytes()[:2]) // SOI
	out.Write([]byte{0xFF, 0xE1})
	_ = binary.Write(&out, binary.BigEndian, uint16(len(app1)+2))
	out.Write(app1)
	out.Write(img.Bytes()[2:])
	return out.Bytes()
}
