package size

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"
	"strconv"
	"strings"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

type sizesExtractor struct {
	logger *l.Logger
}

func (cd *sizesExtractor) Decorate(in exif_core.RawItemRW) (exif_core.RawItemRW, error) {
	w, h := firstSize(
		in,
		"ImageWidth", "ImageHeight",
		"ExifImageWidth", "ExifImageHeight",
		"PixelXDimension", "PixelYDimension",
	)
	if w == 0 || h == 0 {
		w, h = parseImageSize(in.GetExif("ImageSize"))
	}

	if w == 0 || h == 0 {
		// Keep previous size; never write 0×0 (client maps that to a square 1×1).
		return in, nil
	}

	if needsSwap(in.GetExif("Orientation")) || isRotatedVideo(in.GetExif("Rotation")) {
		w, h = h, w
	}

	size := in.GetSize()
	if size.W == w && size.H == h {
		return in, nil
	}

	in.SetSize(api.Size{W: w, H: h})
	in.SetRatio(api.GetRatio(in.GetSize()))

	return in, nil
}

func firstSize(in exif_core.RawItemRW, keys ...string) (int, int) {
	for i := 0; i+1 < len(keys); i += 2 {
		w, h := parseSizePair(in.GetExif(keys[i]), in.GetExif(keys[i+1]))
		if w != 0 && h != 0 {
			return w, h
		}
	}
	return 0, 0
}

func parseSizePair(width, height string) (int, int) {
	w, _ := strconv.Atoi(strings.TrimSpace(width))
	h, _ := strconv.Atoi(strings.TrimSpace(height))
	return w, h
}

// ImageSize is often "4032x3024".
func parseImageSize(s string) (int, int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == 'x' || r == 'X' || r == '×'
	})
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSizePair(parts[0], parts[1])
}

// EXIF orientations 5–8 (and their string forms) display with width/height swapped.
func needsSwap(orientation string) bool {
	switch orientation {
	case "5", "6", "7", "8",
		"Mirror horizontal and rotate 270 CW",
		"Rotate 90 CW",
		"Mirror horizontal and rotate 90 CW",
		"Rotate 270 CW":
		return true
	default:
		return false
	}
}

// QuickTime/MP4 store unrotated track dimensions plus a Rotation matrix (phone portrait video = 90).
func isRotatedVideo(rotation string) bool {
	switch strings.TrimSpace(rotation) {
	case "90", "270", "-90":
		return true
	default:
		return false
	}
}

func (cd *sizesExtractor) Stop() {}

func NewSizesProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	processor := &sizesExtractor{logger}

	return chain.NewDecorator(chin, chout, processor)
}
