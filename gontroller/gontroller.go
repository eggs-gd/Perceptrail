package main

import (
	"context"
	"gontroller/pkg/app"
	"gontroller/pkg/importer"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	mainCtx, cancel := context.WithCancelCause(context.Background())

	ctx := app.NewAppContext()
	svc := app.NewSvcContext(mainCtx)

	defer cancel(nil)

	svc.AddService(importer.NewImporterService(ctx))
	//svc.AddService(importer.NewTranscoderService(ctx))
	//svc.AddService(importer.NewMaintenanceService(ctx))

	go svc.StartApp()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-stop
	log.Printf("Chain Sys stop")
	svc.StopApp()

	// // Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// // Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))
}
