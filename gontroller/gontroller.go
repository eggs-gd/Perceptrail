package main

import (
	"gontroller/pkg/app"
	"gontroller/pkg/importer"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	folderPath := os.Getenv("MEDIA_FOLDER")
	if folderPath == "" {
		log.Fatal("MEDIA_FOLDER not set in .env file")
	}

	ctx := app.NewAppContext()
	ctx.AddService(importer.NewImporterService(ctx))
	ctx.StartApp()

	ctx.wg.Add(1)
	importer.Chain(folderPath, &ctx.wg)
	defer importer.Close()

	ctx.wg.Wait()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-stop
	log.Printf("Chain Sys stop")
	ctx.StopApp()

	// // Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// // Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))
}
