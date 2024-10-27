package model

import (
	t "gontroller/pkg/_t"
	"time"
)

type ValidationApi interface {
	GetHash(path string, rawExif t.RawExif, fileSizeBytes uint64, dateTime time.Time) string
	ValidateFile(data t.RawExif) error
}

type ItemsApi interface {
	GetItemPathHash(path string, hashShort string) (ItemDto, error)

	CreateItem(item ItemDto) error

	UpdateItem(item ItemDto) error
}

type FilesApi interface {
	GetFileByPath(path string) (FileDto, error)

	CreateFile(entry t.ItemEntry) (FileDto, error)

	UpdateFile(file FileDto) (FileDto, error)
	UpdateFiles(files []FileDto) ([]FileDto, error)
}
