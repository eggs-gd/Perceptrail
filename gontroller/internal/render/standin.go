package render

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"perceptrail/gontroller/internal/model/dto"
)

// version: what made the renditions — the stand-in; a real renderer is another
// version, and everything is due again
const version = "stand-in-1"

// standIn: the stand-in renderer — the original itself as the one rendition, linked
// into the cache (copied where a link cannot be made: another file system). It
// tests the queue's rules before any codec.
func standIn(cacheDir string, item *dto.ItemDto) ([]dto.RenditionDto, error) {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(item.Path)), ".")
	rel := filepath.Join("r", item.GUID.String(), fmt.Sprintf("%s-0.%s", version, format))
	dst := filepath.Join(cacheDir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	os.Remove(dst) // a rendition left by an attempt that did not finish
	if err := os.Link(item.Path, dst); err != nil {
		if err := copyFile(item.Path, dst); err != nil {
			return nil, err
		}
	}
	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	return []dto.RenditionDto{{GUID: item.GUID, Version: version, Format: format,
		W: item.Size.W, H: item.Size.H, Bytes: info.Size(), Path: rel}}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
