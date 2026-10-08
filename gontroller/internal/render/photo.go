package render

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"perceptrail/gontroller/internal/model/dto"

	_ "golang.org/x/image/webp"
)

// photo: an image's renditions — one per size, the long side at most that size,
// never upscaled — made by vipsthumbnail (libvips) in a process of its own: a
// broken file fails that process, not the server, and ctx kills it. dir: the
// version's directory in the cache; rel: the same, relative to the cache.
//
// Not obvious:
//   - vipsthumbnail shrinks while it loads where the format allows (JPEG): a 12 MP
//     JPEG is never decoded whole for a 400 px tile.
//   - Metadata is stripped (GPS, the camera): a rendition is pixels only.
//   - Sizes go smallest first; once the original is smaller than a size, the larger
//     ones would be the same image — they are not made (a srcset of two equal
//     widths is invalid).
func photo(ctx context.Context, vipsthumbnail, format string, sizes []int, item *dto.ItemDto, dir, rel string) ([]dto.RenditionDto, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var out []dto.RenditionDto
	for _, size := range slices.Sorted(slices.Values(sizes)) {
		name := fmt.Sprintf("%d.%s", size, format)
		dst := filepath.Join(dir, name)
		cmd := exec.CommandContext(ctx, vipsthumbnail, item.Path,
			"--size", fmt.Sprintf("%dx%d>", size, size), "-o", dst+"[Q=80,strip]")
		if msg, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("vipsthumbnail %d: %w: %s", size, err, msg)
		}
		w, h, bytes, err := header(dst)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.RenditionDto{GUID: item.GUID, Size: size, Format: format,
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
