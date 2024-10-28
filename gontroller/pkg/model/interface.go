package model

import (
	t "gontroller/pkg/_t"
)

type ValidationApi interface {
	// GetShortHash the idea is to hash only size in bytes and whole set of metadata fields
	// Means that expecting if something changed - size in bytes will be different
	// If exifdata changed - full hash will be different
	// Not perfect but as another one gate in bunch of sequential checks:
	// - file scanner gate,
	// - short hash gate,
	// - full hash gate
	//GetShortHash(rawExif t.RawExif, fileSizeBytes uint64) string

	ValidateFile(item FileDto, meta t.RawExif) (ItemDto, error)
}

type ItemsApi interface {
	GetItemByGuid(guid string) (ItemDto, error)
	GetItemByPath(path string) (ItemDto, error)
	GetItemByHash(hash string) (ItemDto, error)
	GetItemsByHash(path string) ([]ItemDto, error)

	CreateItem(file FileDto) (ItemDto, error)

	UpdateItem(item ItemDto) (ItemDto, error)
	UpdateItems(items []ItemDto) ([]ItemDto, error)
}

type FilesApi interface {
	GetFileByPath(path string) (FileDto, error)

	CreateFile(entry t.ItemEntry) (FileDto, error)

	UpdateFile(file FileDto) (FileDto, error)
	UpdateFiles(files []FileDto) ([]FileDto, error)
}
