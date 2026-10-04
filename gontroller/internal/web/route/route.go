// Package route: the HTTP API's routes over the model, the perceptors and the
// libraries (Register).
package route

import (
	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"

	"github.com/labstack/echo/v4"
)

// Store: what the routes read of the model
type Store interface {
	model.ItemsApi
	model.FilesApi
	model.MetaApi
}

// LibraryOf: the library an item belongs to, nil for a plain folder's (the server
// gives library.Of)
type LibraryOf func(item *dto.ItemDto) provider.Renditions

// routes: what the handlers share
type routes struct {
	db         Store
	of         LibraryOf
	logger     *l.Logger
	perceptors []api.Perceptor // the ones the client is given, core first
	values     ValuesLoader    // a perceptor's stored values
	epoch      string          // the delta sync's epoch (items)
}

// Register: every route of the API — items, assets, renditions (of: the item's
// library), the perceptors the client is given (perceptors; values loads their
// stored values), the app's info
func Register(e *echo.Echo, db Store, of LibraryOf, info AppInfo, perceptors []api.Perceptor, values ValuesLoader, logger *l.Logger) {
	r := &routes{db: db, of: of, logger: logger, values: values}
	r.registerItems("/items", e)
	r.registerAssets("/assets", e)
	r.registerPerceptors(e, perceptors)
	r.registerRenditions(e)
	RegisterAppRoutes(e, info)
}
