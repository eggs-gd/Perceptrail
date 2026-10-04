package importer

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The import tests run the real chain with a real exiftool: a file is written as
// the real thing its extension says, its content string inside it (the same
// content: the same bytes, the same fingerprint). Two contents are special:
// "BROKEN…" — a JPEG exiftool cannot read (Error: File format error); "NOSIZE…" — a
// JPEG cut after its header (no image size).
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, fixture(path, content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(path, content string) []byte {
	switch {
	case strings.HasPrefix(content, "BROKEN"):
		return append([]byte{0xFF, 0xD8}, content...)
	case strings.HasPrefix(content, "NOSIZE"):
		return append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, content...)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return jpegOf(content)
	case ".nef", ".dng":
		return tiffOf(content) // a RAW is a TIFF; exiftool takes the type from the extension
	case ".mov", ".mp4":
		return append(box("ftyp", []byte("qt  \x00\x00\x00\x00qt  ")), box("free", []byte(content))...)
	case ".heic":
		return append(box("ftyp", []byte("heic\x00\x00\x00\x00mif1heic")), box("free", []byte(content))...)
	case ".xmp":
		return []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
			`<rdf:Description xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:description>` + content +
			`</dc:description></rdf:Description></rdf:RDF></x:xmpmeta>`)
	}
	return []byte(content)
}

// jpegOf: a 4×3 JPEG, its pixels and a comment from content
func jpegOf(content string) []byte {
	img := image.NewGray(image.Rect(0, 0, 4, 3))
	for i := range img.Pix {
		img.Pix[i] = content[i%len(content)]
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		panic(err)
	}
	com := binary.BigEndian.AppendUint16([]byte{0xFF, 0xFE}, uint16(len(content)+2))
	com = append(com, content...)
	raw := b.Bytes()
	return append(append(append([]byte{}, raw[:2]...), com...), raw[2:]...) // after SOI
}

// tiffOf: a 4×3 TIFF (little-endian) with content as its ImageDescription
func tiffOf(content string) []byte {
	desc := append([]byte(content), 0)
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.Write([]byte{'I', 'I', 42, 0, 8, 0, 0, 0})
	w(uint16(3))
	w([]uint16{256, 3})
	w(uint32(1))
	w(uint32(4)) // ImageWidth
	w([]uint16{257, 3})
	w(uint32(1))
	w(uint32(3)) // ImageLength
	w([]uint16{270, 2})
	w(uint32(len(desc)))
	w(uint32(8 + 2 + 3*12 + 4))
	w(uint32(0))
	b.Write(desc)
	return b.Bytes()
}

// box: an ISO media box (QuickTime, HEIF)
func box(typ string, payload []byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, uint32(8+len(payload))), append([]byte(typ), payload...)...)
}
