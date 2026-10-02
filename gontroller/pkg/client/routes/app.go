package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// AppInfo: what the client needs to know about the server — its version and the
// mode (debug: the gallery shows its debug marks and logs everything; release: not),
// so one client build serves both
type AppInfo struct {
	Version string `json:"version"`
	Mode    string `json:"mode"`
}

func RegisterAppRoutes(e *echo.Echo, info AppInfo) {
	e.GET("/app", func(c echo.Context) error {
		return c.JSON(http.StatusOK, info)
	})
}
