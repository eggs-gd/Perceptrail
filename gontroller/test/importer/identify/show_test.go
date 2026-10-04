package identify_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A RAW with nothing else to show: its embedded preview is extracted into the cache
// and gets the RAW's orientation (or a portrait shot shows on its side)
func TestEmbeddedPreview(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not installed")
	}
	raw := filepath.Join(t.TempDir(), "r.NEF")
	if err := os.WriteFile(raw, rawWithThumb(6), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	it := identifyGroup(t, cache, raw)
	if it == nil {
		t.Fatal("no item")
	}
	if want := filepath.Join(cache, "previews", it.Item.Guid, "embedded.jpg"); it.Item.PreviewPath != want || it.Item.PreviewMime != "image/jpeg" {
		t.Fatalf("preview %q %q, want %q", it.Item.PreviewPath, it.Item.PreviewMime, want)
	}
	out, err := exec.Command("exiftool", "-s3", "-n", "-Orientation", it.Item.PreviewPath).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "6" {
		t.Errorf("preview orientation %q, want 6 (the RAW's)", got)
	}
}

// rawWithThumb: a TIFF (little-endian) as a camera's RAW is: IFD0 the image (4×3,
// its orientation), IFD1 an embedded JPEG — exiftool takes the type from the
// extension (.NEF) and sees the embedded one as ThumbnailImage
func rawWithThumb(orientation uint16) []byte {
	var thumb bytes.Buffer
	if err := jpeg.Encode(&thumb, image.NewGray(image.Rect(0, 0, 16, 12)), nil); err != nil {
		panic(err)
	}
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	entry := func(tag, typ uint16, count, value uint32) { w(tag); w(typ); w(count); w(value) }
	b.Write([]byte{'I', 'I', 42, 0, 8, 0, 0, 0})
	ifd1 := 8 + 2 + 3*12 + 4
	data := ifd1 + 2 + 2*12 + 4
	w(uint16(3))
	entry(256, 3, 1, 4)                   // ImageWidth
	entry(257, 3, 1, 3)                   // ImageLength
	entry(274, 3, 1, uint32(orientation)) // Orientation
	w(uint32(ifd1))
	w(uint16(2))
	entry(513, 4, 1, uint32(data))        // JPEGInterchangeFormat
	entry(514, 4, 1, uint32(thumb.Len())) // JPEGInterchangeFormatLength
	w(uint32(0))
	b.Write(thumb.Bytes())
	return b.Bytes()
}
