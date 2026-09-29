package client

import (
	"context"
	"net/http"
	"perceptrail/gontroller/pkg/client/routes"

	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type webService struct {
	logger *l.Logger
	cfg    ServerConfig
}

// NewWebService checks cfg (see ServerConfig) and fills its defaults.
func NewWebService(cfg ServerConfig, logger *l.Logger) (*webService, error) {
	cfg, err := cfg.withDefaults()
	if err != nil {
		return nil, err
	}
	return &webService{logger, cfg}, nil
}

func (s *webService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: s.cfg.AllowedOrigins,
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	routes.RegisterItemsRoutes("/items", e, s.logger)
	routes.RegisterAssetsRoutes("/assets", e, s.logger)

	e.Logger.Fatal(e.Start(s.cfg.Addr()))

	<-ctx.Done()
}
