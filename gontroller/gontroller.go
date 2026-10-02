package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/client"
	"perceptrail/gontroller/pkg/client/routes"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/scan"
	"syscall"

	"github.com/eggs-gd/go-exiftool"
	l "github.com/eggs-gd/perceplib/logger"
)

func main() {

	mainCtx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	log.Printf("gontroller %s", app.Version)

	ctx := app.NewAppContext()
	if err := model.Configure(ctx.Config().Database); err != nil {
		log.Fatalf("Database: %v", err)
	}
	if ctx.Config().Exiftool != "" {
		exiftool.Exec = ctx.Config().Exiftool
	}
	// Release: Info and up — no SQL (logged at Debug), no per-request lines
	if !ctx.Config().Debug() {
		ctx.SetLogLevel(l.InfoLevel)
	}
	log.Printf("mode: %s", ctx.Config().Mode)
	svc := app.NewSvcContext()

	err := plugins.Pm.LoadPlugins(ctx)
	if err != nil {
		log.Fatalf("Failed to load plugins: %v", err)
	}

	svc.AddService(scan.NewImporterService(ctx))
	web, err := client.NewWebService(ctx.Config().Server, routes.AppInfo{Version: app.Version, Mode: ctx.Config().Mode}, plugins.Pm.ClientPerceptors(), plugins.Pm.LoadValues, ctx.Logger(string(app.LogHTTP)))
	if err != nil {
		log.Fatalf("Server: %v", err)
	}
	svc.AddService(web)
	//svc.AddService(importer.NewMaintenanceService(ctx)) // later

	go svc.RunApp(mainCtx)

	<-stop
	log.Printf("Chain Sys stop")
}
