// The HTTP API over a real model: the routes as the server registers them, read
// through their JSON — the client's contract
package route_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/web/route"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
	"github.com/eggs-gd/perceplib/api"

	"github.com/labstack/echo/v4"
)

// One sqlite database for the package (the model keeps a single connection)
var testDB *model.Proxy

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "route-test")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), nil, 0o644); err != nil {
		panic(err)
	}
	cfg, err := config.Read(filepath.Join(dir, "config.yml")) // the database in dir
	if err != nil {
		panic(err)
	}
	if testDB, err = model.Open(cfg, l.NewLogger(l.ErrorLevel, &tree.Decorator{})); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// server: the routes as the server registers them; of gives an item's library
// (nil: only plain folders), perceptors the ones the client is given
func server(of route.LibraryOf, perceptors ...api.Perceptor) *echo.Echo {
	if of == nil {
		of = func(*dto.ItemDto) route.Library { return nil }
	}
	e := echo.New()
	route.Register(e, testDB, of, route.AppInfo{}, perceptors, nil, l.NewLogger(l.FatalLevel, &tree.Decorator{}))
	return e
}

// get: a GET through the routes
func get(e *echo.Echo, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
