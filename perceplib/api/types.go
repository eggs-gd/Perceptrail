package api

import "time"

type RawExif map[string][]byte

type Size struct {
	W int
	H int
}

type ExifProvider interface {
	//returns exifdata with given key from main file
	//todo add support for sidecars
	GetExif(key string) string
}

type ItemDataProvider interface {
	GetGuid() string
	GetDate() time.Time
	GetSize() Size
	GetRatio() Size
}

type ItemDataEditor interface {
	SetDate(date time.Time)
	SetSize(size Size)
	SetRatio(ratio Size)
}

type RawItemR interface {
	ItemDataProvider
	ExifProvider
}
