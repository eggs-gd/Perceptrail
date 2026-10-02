package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/client"
	"perceptrail/gontroller/pkg/client/routes"
	"perceptrail/gontroller/pkg/importer"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/providers/apple"
	"perceptrail/gontroller/pkg/providers/apple/photokit"
	"perceptrail/gontroller/pkg/providers/folder"
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

	// The providers, in the order the switch asks them; the plain folder last (it
	// takes what nobody claimed). Apple Photos: its library's DB and files; PhotoKit
	// on demand (macOS).
	var ps []providers.Provider
	if ctx.Config().Providers.Enabled("apple") {
		ps = append(ps, apple.New(ctx.Config().Path, photokit.Library{},
			model.NewProxy(ctx.Logger(string(app.LogDB))), ctx.Logger(string(app.LogImporter))))
	}
	ps = append(ps, folder.New())
	providers.Enable(ps...)

	importer := importer.NewImporterService(ctx)
	svc.AddService(importer)
	for _, p := range ps {
		p.Start(mainCtx, importer.Refresh)
	}
	web, err := client.NewWebService(ctx.Config().Server, routes.AppInfo{Version: app.Version, Mode: ctx.Config().Mode}, plugins.Pm.ClientPerceptors(), plugins.Pm.LoadValues, ctx.Logger(string(app.LogHTTP)))
	if err != nil {
		log.Fatalf("Server: %v", err)
	}
	svc.AddService(web)
	//svc.AddService(importer.NewMaintenanceService(ctx)) // later

	go svc.RunApp(mainCtx)

	// The main thread serves PhotoKit's main queue until a stop (macOS; elsewhere: waits)
	done := make(chan struct{})
	go func() { <-stop; close(done) }()
	photokit.RunMain(done)
	log.Printf("Chain Sys stop")
}
