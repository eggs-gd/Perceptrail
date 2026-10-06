package api

import "time"

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
	// GetDuration: a video's length, seconds; 0: not a video or unknown
	GetDuration() float64
	// The perceptors' stored values (Store[T].Put / Get go through it)
	ValueCarrier
}

type RawItemR interface {
	ItemDataProvider
	ExifProvider
}

type RawExif map[string][]byte

type Size struct {
	W int
	H int
}
