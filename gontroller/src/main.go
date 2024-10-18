package main

import (
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	exiftool "gontroller/src/pkg/exif"
)

func main() {
	id := uuid.New()
	log.Println(id.String())

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	folderPath := os.Getenv("MEDIA_FOLDER")
	if folderPath == "" {
		log.Fatal("MEDIA_FOLDER not set in .env file")
	}

	exiftool.Process(folderPath)
	//playground.Run(folderPath)

	// // Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// // Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))
}
