package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/client"
	"perceptrail/gontroller/pkg/scan"
	"perceptrail/perceptors/lib/metadata"
	"syscall"
)

func main() {
	meta := metadata.NewMetadata("Example", "Example plugin", "1.0.0")

	mainCtx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx := app.NewAppContext()
	ctx.SetLogLevel(app.WarnLevel)
	svc := app.NewSvcContext()

	ctx.SetLogLevel(app.InfoLevel)

	svc.AddService(scan.NewImporterService(ctx))
	svc.AddService(client.NewWebService(ctx))
	//svc.AddService(importer.NewMaintenanceService(ctx)) // later

	go svc.RunApp(mainCtx)

	<-stop
	log.Printf("Chain Sys stop")
}
