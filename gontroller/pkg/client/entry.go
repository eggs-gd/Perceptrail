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
	perceptors []api.Perceptor
}

// NewWebService checks cfg (see ServerConfig) and fills its defaults. perceptors:
// the loaded ones — the navigators among them get the /perceptors routes.
func NewWebService(cfg ServerConfig, perceptors []api.Perceptor, logger *l.Logger) (*webService, error) {
	cfg, err := cfg.withDefaults()
	if err != nil {
		return nil, err
	}
	return &webService{logger, cfg, perceptors}, nil
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
	routes.RegisterPerceptorsRoutes(e, s.perceptors, s.logger)

	e.Logger.Fatal(e.Start(s.cfg.Addr()))

	<-ctx.Done()
}
