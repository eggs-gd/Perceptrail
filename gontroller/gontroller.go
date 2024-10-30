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
	defer cancel(nil)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx := app.NewAppContext()
	svc := app.NewSvcContext()

	svc.AddService(importer.NewImporterService(ctx))
	//svc.AddService(importer.NewWebService(ctx)) // later
	//svc.AddService(importer.NewMaintenanceService(ctx)) // later

	go svc.RunApp(mainCtx)

	<-stop
	log.Printf("Chain Sys stop")

	// // Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// // Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))
}
