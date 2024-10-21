package t

import "gontroller/ext/tsugor"

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

type SrcSetW int

const (
	P240  SrcSetW = 240
	P320  SrcSetW = 320
	P720  SrcSetW = 720
	P1280 SrcSetW = 1280
	P1920 SrcSetW = 1920
)

type SrcSet []struct {
	path ItemPath
	w    SrcSetW
}

type ItemInfo struct {
	Exif     ItemExif
	OrigSize Size // Real Size of decoded image
	Srcset   SrcSet
}

type FromPathToExif tsugor.Decorator[ItemPath, ItemExif]

type FromExifToItem tsugor.Decorator[ItemExif, ItemInfo]
