package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"perceptrail/gontroller/internal/model/dto"
)

// version: what made the renditions — the stand-in; a real renderer is another
// version, and everything is due again
const version = "stand-in-1"

// standIn: the stand-in renderer — the original itself as the one rendition, a
// symbolic link in the cache. It tests the queue's rules before any codec.
//
// Not obvious: never a hard link — it changes the original's link count and ctime,
// and an original may be in a Photos library (read only, always), even one no
// provider claims; never a copy — it doubles the library on disk.
func standIn(cacheDir string, item *dto.ItemDto) ([]dto.RenditionDto, error) {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(item.Path)), ".")
	rel := filepath.Join("r", item.GUID.String(), fmt.Sprintf("%s-0.%s", version, format))
	dst := filepath.Join(cacheDir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	os.Remove(dst) // a rendition left by an attempt that did not finish
	if err := os.Symlink(item.Path, dst); err != nil {
		return nil, err
	}
	info, err := os.Stat(dst) // through the link: the original must be there
	if err != nil {
		return nil, err
	}
	return []dto.RenditionDto{{GUID: item.GUID, Version: version, Format: format,
		W: item.Size.W, H: item.Size.H, Bytes: info.Size(), Path: rel}}, nil
}
