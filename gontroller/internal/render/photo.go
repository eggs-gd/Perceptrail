package render

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"

	"perceptrail/gontroller/internal/model/dto"

	_ "golang.org/x/image/webp"
)

// vips: libvips' vipsthumbnail as this server runs it — the format and the sizes the
// config asks for, below the server (proc)
type vips struct {
	bin    string
	format string
	sizes  []int // smallest first (the config sorts them)
	proc   proc
}

// quality: of every rendition (the encoder's 0–100)
const quality = 80

// photo: an image's renditions in to — one per size, the long side at most that
// size, never upscaled — made by vipsthumbnail in a process of its own: a broken
// file fails that process, not the server, and ctx kills it. src: the image (the
// item's main file, or a video's poster).
//
// Not obvious:
//   - vipsthumbnail shrinks while it loads where the format allows (JPEG): a 12 MP
//     JPEG is never decoded whole for a 400 px tile.
//   - Metadata is stripped (GPS, the camera): a rendition is pixels only.
//   - Each is written beside its name (<size>.part.<format>: the format is the
//     extension) and renamed once whole: a rendition the gallery serves is never
//     half of a new one (a render stopped, failed).
//   - Sizes go smallest first (the config sorts them); once a rendition is the
//     original's own size, the larger ones would be the same image — not made (a
//     srcset of two equal widths is invalid): smaller than its size, or no bigger
//     than the one before (an original exactly as big as a size).
func (v vips) photo(ctx context.Context, src string, to itemDir) ([]dto.RenditionDto, error) {
	if err := os.MkdirAll(to.abs, 0o755); err != nil {
		return nil, err
	}
	var out []dto.RenditionDto
	for _, size := range v.sizes {
		name := fmt.Sprintf("%d.%s", size, v.format)
		part := to.path(fmt.Sprintf("%d.part.%s", size, v.format))
		defer os.Remove(part) // a part not renamed: nothing of it stays
		cmd := v.proc.command(ctx, v.bin, src,
			"--size", fmt.Sprintf("%dx%d>", size, size), "-o", fmt.Sprintf("%s[Q=%d,strip]", part, quality))
		if msg, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("vipsthumbnail %d: %w: %s", size, err, msg)
		}
		w, h, bytes, err := header(part)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 && max(w, h) <= max(out[len(out)-1].W, out[len(out)-1].H) {
			break // the one before is the original's size already
		}
		if err := os.Rename(part, to.path(name)); err != nil {
			return nil, err
		}
		out = append(out, dto.RenditionDto{GUID: to.guid, Size: size, Format: v.format, Role: dto.RoleStill,
			W: w, H: h, Bytes: bytes, Path: to.listed(name)})
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
func (v vips) runs(ctx context.Context) error {
	return exec.CommandContext(ctx, v.bin, "--vips-version").Run()
}
