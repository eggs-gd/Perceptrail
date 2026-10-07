package api

import "time"

type ExifProvider interface {
	//returns exifdata with given key from main file
	//todo add support for sidecars
	GetExif(key string) string
}

type ItemDataProvider interface {
	GetGUID() GUID
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

// GUID: an item's identity, the same from the core through every perceptor; it
// never changes (a moved file keeps its item's GUID)
type GUID string

// NilGUID: a GUID in its format that names no item
const NilGUID GUID = "00000000-0000-0000-0000-000000000000"

type RawExif map[string][]byte

type Size struct {
	W int
	H int
}

func (g GUID) String() string { return string(g) }
