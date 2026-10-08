package render

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
	_ "golang.org/x/image/webp"
)

// quality: of every rendition (the encoder's 0–100)
const quality = 80

// photo: an image's renditions — one per size, the long side at most that size,
// never upscaled — made by vipsthumbnail (libvips) in a process of its own: a
// broken file fails that process, not the server, and ctx kills it. dir: the item's
// directory in the cache; rel: the same, relative to the cache; src: the image (the
// item's main file, or a video's poster).
//
// Not obvious:
//   - vipsthumbnail shrinks while it loads where the format allows (JPEG): a 12 MP
//     JPEG is never decoded whole for a 400 px tile.
//   - Metadata is stripped (GPS, the camera): a rendition is pixels only.
//   - Sizes go smallest first (the config sorts them); once a rendition is the
//     original's own size, the larger ones would be the same image — not made (a
//     srcset of two equal widths is invalid): smaller than its size, or no bigger
//     than the one before (an original exactly as big as a size).
func photo(ctx context.Context, vipsthumbnail, format string, sizes []int, guid api.GUID, src, dir, rel string) ([]dto.RenditionDto, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var out []dto.RenditionDto
	for _, size := range sizes {
		name := fmt.Sprintf("%d.%s", size, format)
		dst := filepath.Join(dir, name)
		cmd := exec.CommandContext(ctx, vipsthumbnail, src,
			"--size", fmt.Sprintf("%dx%d>", size, size), "-o", fmt.Sprintf("%s[Q=%d,strip]", dst, quality))
		if msg, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("vipsthumbnail %d: %w: %s", size, err, msg)
		}
		w, h, bytes, err := header(dst)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 && max(w, h) <= max(out[len(out)-1].W, out[len(out)-1].H) {
			os.Remove(dst) // the one before is the original's size already
			break
		}
		out = append(out, dto.RenditionDto{GUID: guid, Size: size, Format: format, Role: dto.RoleStill,
			W: w, H: h, Bytes: bytes, Path: filepath.Join(rel, name)})
		if max(w, h) < size {
			break // the original is smaller: a larger size is the same image
		}
	}
	return out, nil
}

// header: a rendition's pixel size and bytes
func header(path string) (w, h int, bytes int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		return 0, 0, 0, err
	}
	return cfg.Width, cfg.Height, info.Size(), nil
}

// runs: vipsthumbnail is there and runs
func runs(ctx context.Context, vipsthumbnail string) error {
	return exec.CommandContext(ctx, vipsthumbnail, "--vips-version").Run()
}
