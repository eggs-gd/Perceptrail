// Package fake: stand-ins for the outside world in the integration tests (what the
// server cannot have in CI: Photos)
package fake

import (
	"errors"
	"os"
	"path/filepath"
)

// Photos stands in for Photos (PhotoKit): asked for a rendition, it writes the file where
// Photos would (the naming layout) — or fails
type Photos struct {
	Root  string
	Asked []string
	Fail  bool
	Draw  bool // draw from a local original: no file, only the JPEG
}

// Photos' video delivery modes (photokit)
const videoOriginal, videoFast = 1, 3

func (f *Photos) Image(uuid string, size int) ([]byte, error) {
	if f.Draw {
		f.Asked = append(f.Asked, "drawn")
		return []byte("JPEG"), nil
	}
	return []byte("JPEG"), f.put(uuid, "_1_102_o.jpeg")
}

func (f *Photos) Video(uuid string, mode int) (string, error) {
	name := "_2_201_o.mov"
	switch mode {
	case videoFast:
		name = "_2_4_o.mp4"
	case videoOriginal:
		name = "_ORIGINAL.mov"
	}
	return filepath.Join(f.Root, "resources", "derivatives", uuid[:1], uuid+name), f.put(uuid, name)
}

func (f *Photos) Full(uuid string) ([]byte, error) {
	f.Asked = append(f.Asked, "full")
	return []byte("FULL JPEG"), nil
}

func (f *Photos) Live(uuid string) error { return f.put(uuid, "_2_101_o.mov") }

func (f *Photos) Authorize() bool { return true }

func (f *Photos) put(uuid, name string) error {
	f.Asked = append(f.Asked, name)
	if f.Fail {
		return errors.New("offline")
	}
	p := filepath.Join(f.Root, "resources", "derivatives", uuid[:1], uuid+name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(name), 0o644)
}
