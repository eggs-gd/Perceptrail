package main

import (
	"context"
	"gontroller/pkg/app"
	"gontroller/pkg/client"
	"gontroller/pkg/scan"
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

	ctx.SetLogLevel(app.InfoLevel)

	svc.AddService(scan.NewImporterService(ctx))
	svc.AddService(client.NewWebService(ctx))
	//svc.AddService(importer.NewMaintenanceService(ctx)) // later

	go svc.RunApp(mainCtx)

	<-stop
	log.Printf("Chain Sys stop")
}
