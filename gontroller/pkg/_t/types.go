package t

type ItemPath string

type RawExif map[string][]byte

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
