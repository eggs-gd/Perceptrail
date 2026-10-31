package render

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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

// photo: an image's renditions in to, smallest first — one per size, the long side
// at most that size, never upscaled — made by vipsthumbnail in a process of its own:
// a broken file fails that process, not the server, and ctx kills it. src: the image
// (source(): the item's main file or its preview, or a video's poster).
//
// Not obvious:
//   - The largest size is made from src, each smaller one from the size above it:
//     src is read once. vipsthumbnail shrinks while it loads where the format allows
//     (a JPEG is never decoded whole for a 400 px tile), a HEIC it decodes whole on
//     every read (measured: findings "Photo renditions: what costs").
//   - Metadata is stripped (GPS, the camera): a rendition is pixels only.
//   - Each is written beside its name (<size>.part.<format>: the format is the
//     extension) and renamed once whole: a rendition the gallery serves is never
//     half of a new one (a render stopped, failed).
//   - An original smaller than a size gives its own size: such a rendition is named
//     by the smallest size it answers, and a size no bigger than the one above is
//     not made — the same image again (a srcset of two equal widths is invalid).
func (v vips) photo(ctx context.Context, src string, to itemDir) ([]dto.RenditionDto, error) {
	if err := os.MkdirAll(to.abs, 0o755); err != nil {
		return nil, err
	}
	var out []dto.RenditionDto
	for i := len(v.sizes) - 1; i >= 0; i-- {
		size := v.sizes[i]
		if len(out) > 0 && max(out[0].W, out[0].H) <= size {
			continue // the one above is this size already
		}
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
		answers := v.answers(max(w, h))
		name := fmt.Sprintf("%d.%s", answers, v.format)
		if err := os.Rename(part, to.path(name)); err != nil {
			return nil, err
		}
		out = append([]dto.RenditionDto{{GUID: to.guid, Size: answers, Format: v.format, Role: dto.RoleStill,
			W: w, H: h, Bytes: bytes, Path: to.listed(name)}}, out...)
		src = to.path(name)
	}
	return out, nil
}

// source: what an item's stills are made of — its preview (what the cheap stage
// found: the group's JPEG, the RAW's embedded one, already turned upright) when its
// long side reaches the largest size, else the original. A RAW's preview is the
// camera's own rendering, and decoding it is a fraction of decoding the RAW, in time
// and memory (findings "Photo renditions: what costs"); an original the browser
// shows is its own preview.
func (v vips) source(item *dto.ItemDto) string {
	if item.PreviewPath == "" || item.PreviewPath == item.Path {
		return item.Path
	}
	f, err := os.Open(item.PreviewPath)
	if err != nil {
		return item.Path
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || max(cfg.Width, cfg.Height) < v.sizes[len(v.sizes)-1] {
		return item.Path
	}
	return item.PreviewPath
}

// answers: the smallest size a rendition of this long side answers — the size it
// was made for, or a smaller one when the image was smaller than that
func (v vips) answers(long int) int {
	for _, size := range v.sizes {
		if long <= size {
			return size
		}
	}
	return v.sizes[len(v.sizes)-1]
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
