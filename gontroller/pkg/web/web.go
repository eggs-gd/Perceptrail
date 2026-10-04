// Package web: the HTTP service (Echo) — the API the client reads; the routes are
// package route
package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/config"
	"perceptrail/gontroller/pkg/perceptor"
	"perceptrail/gontroller/pkg/web/route"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Config: what the HTTP service reads of the config — where it listens, CORS;
// debug logs every request
type Config interface {
	Server() config.Server
	Debug() bool
	Mode() string
}

// Service: the HTTP API — the items, their assets and renditions, the perceptors
// the client is given (perceptor.Client, their values from perceptor.LoadValues),
// renditions on demand from the item's library
type Service struct {
	cfg    Config
	db     route.Store
	logger *l.Logger
}

func New(cfg Config, db route.Store, logger *l.Logger) *Service {
	return &Service{cfg: cfg, db: db, logger: logger}
}

func (s *Service) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: s.cfg.Server().AllowedOrigins,
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
		// The client reads the delta sync's cursor and epoch (/items)
		ExposeHeaders: []string{"X-Sync-Epoch"},
	}))
	// A line per request (every tile image too): debug only
	if s.cfg.Debug() {
		e.Use(middleware.Logger())
	}
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	route.Register(e, s.db, route.AppInfo{Version: app.Version, Mode: s.cfg.Mode()}, perceptor.Client(), perceptor.LoadValues, s.logger)

	go func() {
		if err := e.Start(s.cfg.Server().Addr()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			e.Logger.Fatal(err)
		}
	}()

	// A stop: the server closes (requests in flight get a few seconds)
	<-ctx.Done()
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	if err := e.Shutdown(shutdown); err != nil {
		s.logger.Error("Server shutdown", l.Error(err))
	}
}
