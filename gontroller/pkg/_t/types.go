package t

import (
	"time"
)

type RawExif map[string][]byte

type ItemEntry struct {
	Path     string
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
}

type Size struct {
	W int
	H int
}
