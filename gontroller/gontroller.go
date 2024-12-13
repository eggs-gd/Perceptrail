package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/client"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/scan"
	"syscall"

	l "github.com/dukobpa3/perceplib/logger"
)

func main() {

	mainCtx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx := app.NewAppContext()
	ctx.SetLogLevel(l.WarnLevel)
	//ctx.SetLogLevel(l.InfoLevel)
	svc := app.NewSvcContext()

	err := plugins.Pm.LoadPlugins(ctx)
	if err != nil {
		log.Fatalf("Failed to load plugins: %v", err)
	}

	svc.AddService(scan.NewImporterService(ctx))
	svc.AddService(client.NewWebService(ctx))
	//svc.AddService(importer.NewMaintenanceService(ctx)) // later

	go svc.RunApp(mainCtx)

	<-stop
	log.Printf("Chain Sys stop")
}
