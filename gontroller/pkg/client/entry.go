package client

import (
	"context"
	"gontroller/pkg/app"
	"gontroller/pkg/client/routes"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type webService struct {
	appCtx app.AppContext
}

func NewWebService(ctx app.AppContext) *webService {
	return &webService{ctx}
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

	routes.RegisterItemsRoutes("/items", e)
	routes.RegisterAssetsRoutes("/assets", e)

	e.Logger.Fatal(e.Start(":1323"))

	<-ctx.Done()
}
