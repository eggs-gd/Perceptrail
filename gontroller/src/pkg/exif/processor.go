package exiftool

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var commonArgs []string = []string{
	"-srcfile",
	"@",
	"-r",
	"-json",
}

var genericTags []string = []string{
	//"-FileType",
	"-MIMEType",
	// "-ExifImageWidth",
	// "-ExifImageHeight",
	// "-ImageWidth",
	// "-ImageHeight",
	// "-ThumbnailImageWidth",
	// "-ThumbnailImageHeight",
	// "-DisplayWidth",
	// "-DisplayHeight",
	// "-ImageSize",
	// "-SourceImageWidth",
	// "-SourceImageHeight",
	// "-Duration",
	// "-AvgBitrate",
	// "-VideoCodec",
	// "-AudioCodec",
}

func Process(path string) {
	var et, err = NewServer()
	if err != nil {
		et.Close()
		panic(err)
	}

	var out []byte = make([]byte, 0)
	args := append(genericTags, commonArgs...)
	out, err = et.Command(append(args, path)...)
	if err != nil {
		log.Printf("Stdout %v\n", string(out))
		log.Fatalf("Error Command running: %v\n", err)
		et.Close()
		panic(err)
	}

	var res map[string][]byte = make(map[string][]byte)
	err = Unmarshal(out, res)
	if err != nil {
		log.Fatalf("Error Unmarshaling: %v\n", err)
		et.Close()
		panic(err)
	}

	//log.Printf("Original size: %dx%d", originalWidth, originalHeight)

	outputFile := filepath.Join("./", "media_info.json")
	file, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Fatalf("Stderr Error marshaling JSON: %v\n", err)
		et.Close()
		panic(err)
	}

	err = os.WriteFile(outputFile, file, 0644)
	if err != nil {
		log.Fatalf("Error writing file: %v\n", err)
		et.Close()
		panic(err)
	}

	fmt.Printf("Media info saved to %s\n", outputFile)

	et.Shutdown()

}
