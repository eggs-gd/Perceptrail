package items

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type MediaInfo struct {
	File    string  `json:"file"`
	Sidecar string  `json:"sidecar,omitempty"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Ratio   float64 `json:"ratio"`
}

func Scrape(folderPath string) {
	fileChannel := make(chan string)
	infosChannel := make(chan MediaInfo)
	var wg sync.WaitGroup

	mediaInfos := []MediaInfo{}

	wg.Add(1)
	go getFilesList(folderPath, fileChannel, &wg)

	for filePath := range fileChannel {
		wg.Add(1)
		go processFile(filePath, infosChannel, &wg)
	}

	go watchInfos(&mediaInfos, infosChannel)

	wg.Wait()
	close(infosChannel)

	outputFile := filepath.Join(folderPath, "media_info.json")
	file, err := json.MarshalIndent(mediaInfos, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v\n", err)
	}

	err = os.WriteFile(outputFile, file, 0644)
	if err != nil {
		log.Fatalf("Error writing file: %v\n", err)
	}

	fmt.Printf("Media info saved to %s\n", outputFile)
}

func watchInfos(mediaInfos *[]MediaInfo, infos <-chan MediaInfo) {
	for info := range infos {
		*mediaInfos = append(*mediaInfos, info)
	}
}

func getFilesList(folderPath string, fileChannel chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && isMediaFile(path) {
			fileChannel <- path
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %v: %v\n", folderPath, err)
	}

	close(fileChannel)

}

func processFile(filePath string, infos chan<- MediaInfo, wg *sync.WaitGroup) {
	defer wg.Done()

	dimensions := getDimensions(filePath)
	sidecar := getSidecarFile(filePath)

	info := MediaInfo{
		File:    filePath,
		Sidecar: sidecar,
		Width:   dimensions[0],
		Height:  dimensions[1],
		Ratio:   float64(dimensions[0]) / float64(dimensions[1]),
	}
	infos <- info
}

func isMediaFile(filePath string) bool {
	ext := filepath.Ext(filePath)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".mp4", ".mov", ".avi", ".mkv":
		return true
	}
	return false
}

func getDimensions(filePath string) [2]int {
	fmt.Printf("Getting dimensions %s\n", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %v: %v\n", filePath, err)
		return [2]int{0, 0}
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Printf("Error decoding image %v: %v\n", filePath, err)
		return [2]int{0, 0}
	}

	bounds := img.Bounds()
	return [2]int{bounds.Dx(), bounds.Dy()}
}

func getSidecarFile(filePath string) string {
	fmt.Printf("Getting sidecars %s\n", filePath)
	xmpFile := filePath[:len(filePath)-len(filepath.Ext(filePath))] + ".xmp"
	if _, err := os.Stat(xmpFile); err == nil {
		return xmpFile
	}

	// Перевірити варіант <file>.<ext>.xmp
	ext := filepath.Ext(filePath)
	baseName := filePath[:len(filePath)-len(ext)]
	sidecarWithExt := baseName + ext + ".xmp"
	if _, err := os.Stat(sidecarWithExt); err == nil {
		return sidecarWithExt
	}

	return ""
}
