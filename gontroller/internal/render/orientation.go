package render

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"image"
	"io"
	"os"

	"perceptrail/gontroller/internal/model/dto"
)

// orientationTag: EXIF Orientation, in IFD0
const orientationTag = 0x0112

// staleOrientation: src is a JPEG whose Orientation tag is not followed
// (dto.StaleOrientation) — read from the file itself, its pixels' size from its
// header and its tag from its EXIF: render does not wait for what the import stored
// of it.
func staleOrientation(src string) bool {
	f, err := os.Open(src)
	if err != nil {
		return false
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil || format != "jpeg" || cfg.Height <= cfg.Width {
		return false
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false
	}
	return dto.StaleOrientation("image/jpeg", jpegOrientation(bufio.NewReader(f)), cfg.Width, cfg.Height)
}

// jpegOrientation: a JPEG's EXIF Orientation, 0 when it has none. The segments up to
// the image data are walked; the first Exif APP1 is read.
func jpegOrientation(r io.Reader) int {
	var soi [2]byte
	if _, err := io.ReadFull(r, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return 0
	}
	for {
		var head [4]byte // 0xFF, the marker, the segment's length (it counts itself)
		if _, err := io.ReadFull(r, head[:]); err != nil || head[0] != 0xFF {
			return 0
		}
		marker, size := head[1], int(binary.BigEndian.Uint16(head[2:]))-2
		if marker == 0xDA || marker == 0xD9 || size < 0 { // the image data: no EXIF before it
			return 0
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(r, data); err != nil {
			return 0
		}
		if marker == 0xE1 && bytes.HasPrefix(data, []byte("Exif\x00\x00")) {
			return tiffOrientation(data[6:])
		}
	}
}

// tiffOrientation: the Orientation in a TIFF block's IFD0 (EXIF is one), 0 if none
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 0
	}
	n := int(order.Uint16(t[ifd:]))
	for i := range n {
		e := ifd + 2 + 12*i // tag, type, count, value
		if e+12 > len(t) {
			return 0
		}
		if order.Uint16(t[e:]) == orientationTag {
			return int(order.Uint16(t[e+8:])) // a SHORT, at the value's start
		}
	}
	return 0
}
