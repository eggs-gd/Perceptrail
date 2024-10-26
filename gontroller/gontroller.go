package main

import (
	"gontroller/pkg/importer"
	"log"
	"os"
	"sync"
)

func main() {
	// id := uuid.New()
	// log.Println(id.String())

	folderPath := os.Getenv("MEDIA_FOLDER")
	if folderPath == "" {
		log.Fatal("MEDIA_FOLDER not set in .env file")
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	importer.Chain(folderPath, wg)

	wg.Wait()

	//exif.Process(folderPath)
	//playground.Run(folderPath)

	// // Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// // Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))
}
