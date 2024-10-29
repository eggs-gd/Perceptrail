package main

import (
	"gontroller/pkg/importer"
	"log"
	"os"
	"sync"
)

type AppContext struct {
	config   *Config
	logger   *log.Logger
	wg       sync.WaitGroup
	services []Service
}

type Service interface {
	Start()
	Stop()
}

func main() {
	folderPath := os.Getenv("MEDIA_FOLDER")
	if folderPath == "" {
		log.Fatal("MEDIA_FOLDER not set in .env file")
	}

	ctx := &AppContext{
		config: &AppConfig,
		logger: log.New(os.Stdout, "app: ", log.LstdFlags),
	}

	ctx.wg.Add(1)
	importer.Chain(folderPath, &ctx.wg)
	defer importer.Close()

	ctx.wg.Wait()

	// // Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// // Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))
}
