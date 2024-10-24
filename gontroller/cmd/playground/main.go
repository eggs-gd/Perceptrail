package playground

import (
	"encoding/json"
	"fmt"
	"gontroller/ext/chain"
	"log"
	"os"
	"path/filepath"
	"sync"
)

func Run(path string) {
	fileChannel := make(chan string)
	infosChannel := make(chan MediaInfo)
	var wg sync.WaitGroup

	mediaInfos := []MediaInfo{}

	wg.Add(1)
	go getFilesList(path, fileChannel, &wg)

	wg.Add(1)
	go chain.DecorateAsync(10, fileChannel, infosChannel, processFile, &wg)

	go watchInfos(&mediaInfos, infosChannel)

	wg.Wait()

	outputFile := filepath.Join(path, "media_info.json")
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
