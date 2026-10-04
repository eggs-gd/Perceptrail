package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"perceptrail/gontroller/internal/app"
	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/importer"
	"perceptrail/gontroller/internal/library"
	"perceptrail/gontroller/internal/library/apple/photokit"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/perceptor"
	"perceptrail/gontroller/internal/web"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// The server: its modules, in the order they start. Each reads what it needs of
// the config (its own Config interface) and gets its logger by name.
func main() {
	log.Printf("gontroller %s", app.Version)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// Release: Info and up — no SQL (logged at Debug), no per-request lines
	logs := l.NewLogger(cfg.LogLevel(), &decorators.GontrollerDecorator{})
	logs.Info("Configuration loaded", l.String("config", cfg.File()), l.String("mode", cfg.Mode()), l.String("data_dir", cfg.DataDir()))

	db, err := model.Open(cfg, logs.Named("db"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := perceptor.Load(cfg, logs.Named("perceptors")); err != nil {
		log.Fatalf("perceptors: %v", err)
	}
	if err := library.Enable(cfg, db, logs.Named("libraries")); err != nil {
		log.Fatalf("libraries: %v", err)
	}

	services := app.NewServices()
	services.Add(library.Service())
	services.Add(importer.New(cfg, db, logs.Named("importer")))
	services.Add(web.New(cfg, db, logs.Named("http")))

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		services.Run(ctx)
		close(stopped)
	}()

	// The main thread serves PhotoKit's main queue until a stop (macOS; elsewhere: waits)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() { <-stop; close(done) }()
	photokit.RunMain(done)

	// The services stop and clean up (the importer closes its exiftool processes, or
	// they outlive us) — at most a while: a step may be inside a long exiftool call
	cancel()
	select {
	case <-stopped:
		// Nothing writes any more: the databases closed (their journals merged)
		perceptor.Close()
		if err := db.Close(); err != nil {
			log.Printf("database not closed: %v", err)
		}
	case <-time.After(10 * time.Second):
		log.Printf("services did not stop in time")
	}
	log.Printf("stopped")
}
