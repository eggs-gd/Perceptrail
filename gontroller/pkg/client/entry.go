package client

import (
	"context"
	"net/http"
	"perceptrail/gontroller/pkg/client/routes"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type webService struct {
	logger     *l.Logger
	cfg        ServerConfig
	app        routes.AppInfo
	perceptors []api.Perceptor
	values     routes.ValuesLoader
	fetcher    routes.Fetcher
}

// NewWebService checks cfg (see ServerConfig) and fills its defaults. perceptors:
// the ones the client is given (/perceptors, /p/:name/order); fetcher: asks Apple
// Photos for renditions on demand (nil: only what is on disk).
func NewWebService(cfg ServerConfig, app routes.AppInfo, perceptors []api.Perceptor, values routes.ValuesLoader, fetcher routes.Fetcher, logger *l.Logger) (*webService, error) {
	cfg, err := cfg.withDefaults()
	if err != nil {
		return nil, err
	}
	return &webService{logger, cfg, app, perceptors, values, fetcher}, nil
}

func (s *webService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: s.cfg.AllowedOrigins,
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
		// The client reads the delta sync's cursor and epoch (/items)
		ExposeHeaders: []string{"X-Sync-Epoch"},
	}))
	// A line per request (every tile image too): debug only
	if s.app.Mode == "debug" {
		e.Use(middleware.Logger())
	}
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	routes.RegisterItemsRoutes("/items", e, s.logger)
	routes.RegisterAssetsRoutes("/assets", e, s.logger)
	routes.RegisterPerceptorsRoutes(e, s.perceptors, s.values, s.logger)
	routes.RegisterAppRoutes(e, s.app)
	routes.RegisterRenditionRoutes(e, s.fetcher, s.logger)

	e.Logger.Fatal(e.Start(s.cfg.Addr()))

	<-ctx.Done()
}
