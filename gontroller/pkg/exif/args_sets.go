package exif

var commonArgs []string = []string{ // all sidecars
	"-srcfile",
	"@",
}

var genericTags []string = []string{ // Generic tags needed for db.Item
	//"-FileType",
	"-MIMEType",
	"-ExifImageWidth",
	"-ExifImageHeight",
	"-ImageWidth",
	"-ImageHeight",
	// "-ThumbnailImageWidth",
	// "-ThumbnailImageHeight",
	// "-DisplayWidth",
	// "-DisplayHeight",
	// "-ImageSize",
	// "-SourceImageWidth",
	// "-SourceImageHeight",
	"-Duration",
	"-AvgBitrate",
	"-VideoCodec",
	"-AudioCodec",
}
