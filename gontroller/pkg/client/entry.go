package client

import (
	"context"
	"net/http"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/client/routes"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

type webService struct {
	logger *zap.Logger
	appCtx app.AppContext
}

func NewWebService(ctx app.AppContext) *webService {
	logger := ctx.Logger("ImporterService")
	return &webService{logger, ctx}
}

func (s *webService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	routes.RegisterItemsRoutes("/items", e, s.logger)
	routes.RegisterAssetsRoutes("/assets", e, s.logger)

	e.Logger.Fatal(e.Start(":1323"))

	<-ctx.Done()
}
