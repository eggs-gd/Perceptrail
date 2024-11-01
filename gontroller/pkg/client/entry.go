package client

import (
	"context"
	"gontroller/pkg/app"
	"net/http"

	"github.com/labstack/echo/v4"
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

	// https://echo.labstack.com/docs/quick-start
	// https://habr.com/en/companies/ozonbank/articles/817381/
	// https://github.com/go-chi/chi
	/*
		e.POST("/users", saveUser)
		e.GET("/users/:id", getUser)
		e.PUT("/users/:id", updateUser)
		e.DELETE("/users/:id", deleteUser)

		// e.GET("/users/:id", getUser)
		func getUser(c echo.Context) error {
		  	// User ID from path `users/:id`
		  	id := c.Param("id")
			return c.String(http.StatusOK, id)
		}

		/show?team=x-men&member=wolverine
		//e.GET("/show", show)
		func show(c echo.Context) error {
			// Get team and member from the query string
			team := c.QueryParam("team")
			member := c.QueryParam("member")
			return c.String(http.StatusOK, "team:" + team + ", member:" + member)
		}
	*/

	// Вказуємо шлях і функцію обробник
	// http.HandleFunc("/scan", scanFolder)

	// Запускаємо сервер на порту 8080
	// port := "8080"
	// fmt.Printf("Starting server on port %s...\n", port)
	// log.Fatal(http.ListenAndServe(":"+port, nil))

	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.Logger.Fatal(e.Start(":1323"))

	<-ctx.Done()
}
