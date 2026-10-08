package render

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
//   - Sizes go smallest first (the config sorts them); once a rendition is the
//     original's own size, the larger ones would be the same image — not made (a
//     srcset of two equal widths is invalid): smaller than its size, or no bigger
//     than the one before (an original exactly as big as a size).
func photo(ctx context.Context, vipsthumbnail, format string, sizes []int, item *dto.ItemDto, dir, rel string) ([]dto.RenditionDto, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var out []dto.RenditionDto
	for _, size := range sizes {
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
		if len(out) > 0 && max(w, h) <= max(out[len(out)-1].W, out[len(out)-1].H) {
			os.Remove(dst) // the one before is the original's size already
			break
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

// vipsVersion: the libvips behind vipsthumbnail, major.minor ("8.18") — it runs, or
// an error
func vipsVersion(ctx context.Context, vipsthumbnail string) (string, error) {
	out, err := exec.CommandContext(ctx, vipsthumbnail, "--vips-version").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out)) // "libvips 8.18.7"
	if len(fields) < 2 {
		return "", fmt.Errorf("no version in %q", out)
	}
	parts := strings.SplitN(fields[len(fields)-1], ".", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("no major.minor in %q", out)
	}
	return parts[0] + "." + parts[1], nil
}
