package t

import (
	"os"
	"time"
)

const PerceptrailPathFieldName string = "__perceptrail_file_path"

type RawExif map[string][]byte

type ItemEntry struct {
	Path     string
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
}

func NewItemEntryFromDirEntry(path string, dirEntry os.DirEntry) ItemEntry {
	i := ItemEntry{
		Path: path,
		Name: dirEntry.Name(),
	}

	info, err := dirEntry.Info()
	if err == nil {
		i.Size = info.Size()
		i.ModTime = info.ModTime()
	}
	return i
}

type Size struct {
	W int
	H int
}
