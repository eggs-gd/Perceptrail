package identify

import (
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	l "github.com/eggs-gd/go-zap-decor"
	_ "golang.org/x/image/webp"
)

// Large originals must not turn the cheap stage into image rendering. When a color
// cannot be sampled cheaply, the placeholder falls back to a stable muted color.
const previewColorMaxBytes = 4 << 20

func previewColor(seed, path, mime string, logger *l.Logger) string {
	if path == "" {
		return ""
	}
	fallback := fallbackColor(seed)
	if !strings.HasPrefix(mime, "image/") {
		return fallback
	}
	if strings.Contains(path, "://") {
		return fallback
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > previewColorMaxBytes {
		return fallback
	}
	file, err := os.Open(path)
	if err != nil {
		return fallback
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		if logger != nil {
			logger.Debug("Preview color not decoded", l.String("file", path), l.Error(err))
		}
		return fallback
	}
	if color := averageColor(img); color != "" {
		return color
	}
	return fallback
}

func fallbackColor(seed string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	hue := h.Sum32() % 360
	return hslColor(hue, 22, 26)
}

func hslColor(h, s, l uint32) string {
	chroma := (100 - abs(int(2*l)-100)) * s / 100
	x := chroma * uint32(60-abs(int(h%120)-60)) / 60
	m := l - chroma/2
	var r, g, b uint32
	switch {
	case h < 60:
		r, g = chroma, x
	case h < 120:
		r, g = x, chroma
	case h < 180:
		g, b = chroma, x
	case h < 240:
		g, b = x, chroma
	case h < 300:
		r, b = x, chroma
	default:
		r, b = chroma, x
	}
	return fmt.Sprintf("#%02x%02x%02x", (r+m)*255/100, (g+m)*255/100, (b+m)*255/100)
}

func abs(n int) uint32 {
	if n < 0 {
		return uint32(-n)
	}
	return uint32(n)
}

func averageColor(img image.Image) string {
	bounds := img.Bounds()
	if bounds.Empty() {
		return ""
	}
	stepX := max(1, bounds.Dx()/64)
	stepY := max(1, bounds.Dy()/64)
	var r, g, b, n uint64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			if ca == 0 {
				continue
			}
			r += uint64(cr)
			g += uint64(cg)
			b += uint64(cb)
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", byte((r/n)>>8), byte((g/n)>>8), byte((b/n)>>8))
}
