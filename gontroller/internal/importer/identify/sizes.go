package identify

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"

	"perceptrail/gontroller/internal/model/dto"

	_ "golang.org/x/image/webp"
)

// SizesStore: what sizes writes — the files' pixels and codecs
type SizesStore interface {
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
}

// setSizes: the original's size comes from its metadata (the source's first: the
// Photos DB size is oriented); images from their header — no decoding
func setSizes(it *draft) {
	for i, f := range it.Files {
		if f.Role == dto.RoleMeta {
			continue
		}
		if it.Exif[i] != nil {
			f.Codec = string(it.Exif[i]["CompressorID"])
		}
		// The metadata describes the original: a derivative standing in as the main
		// file (a cloud-only asset) has its own size
		if i == 0 && f.Role == dto.RoleOriginal {
			w, _ := strconv.Atoi(mainTag(it, "ImageWidth"))
			h, _ := strconv.Atoi(mainTag(it, "ImageHeight"))
			if w > 0 && h > 0 {
				f.Width, f.Height = w, h
				continue
			}
		}
		if w, h, ok := headerSize(f.Path); ok {
			f.Width, f.Height = w, h
		}
	}
}

// mainTag: a tag of the main file — the source's metadata first, then its EXIF
func mainTag(it *draft, tag string) string {
	if v, ok := it.Meta[tag]; ok {
		return string(v)
	}
	if it.Exif[0] != nil {
		return string(it.Exif[0][tag])
	}
	return ""
}

// headerSize reads an image's size from its header (JPEG, PNG, GIF, WebP)
func headerSize(path string) (int, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}
