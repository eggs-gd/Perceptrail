package t

import (
	"gontroller/ext/tsugor"
)

type ItemPath string

type Size struct {
	W int
	H int
}

type ItemExif struct {
	Path     string
	MimeType string
	Size     Size // ItemWidth/ItemHeight
	ExifSize Size // ExifItemWidth/ExifItemHeight
}

type ItemInfo struct {
	ItemExif
	OrigSize Size // Real Size of decoded image
}

type FromPathToExif tsugor.Decorator[ItemPath, ItemExif]

type FromExifToItem tsugor.Decorator[ItemExif, ItemInfo]
