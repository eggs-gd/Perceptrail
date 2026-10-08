// Package route: the HTTP API's routes over the model, the perceptors and the
// libraries (Register).
package route

import (
	"time"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
	"github.com/labstack/echo/v4"
)

// Store: what the routes read of the model (and the delta sync's epoch they keep)
type Store interface {
	GetItemByGUID(guid api.GUID) (*dto.ItemDto, error)
	GetItemsInStates(states ...dto.ItemState) ([]*dto.ItemDto, error)
	CountItemsInStates(states ...dto.ItemState) (int64, error)
	StreamAllItems(fn func(dto.StoredItem) error) error
	StreamItemsSince(since time.Time, fn func(dto.StoredItem) error) error
	Renditions(guid api.GUID) ([]dto.RenditionDto, error)
	GetFileByID(id uint) (*dto.FileDto, error)
	GetLinkedFiles(guid api.GUID) ([]*dto.FileDto, error)
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
}

// Library: what the routes ask of an item's library — what may be asked for on
// demand, and a rendition
type Library interface {
	Levels(item *dto.ItemDto) []string
	Rendition(item *dto.ItemDto, level string, opt provider.Options) (provider.Rendition, error)
}

// LibraryOf: the library an item belongs to, nil for a plain folder's (the server
// gives library.Of)
type LibraryOf func(item *dto.ItemDto) Library

// routes: what the handlers share
type routes struct {
	db         Store
	of         LibraryOf
	cache      string // the data's cache: renditions are under it
	logger     *l.Logger
	perceptors []api.Perceptor // the ones the client is given, core first
	values     ValuesLoader    // a perceptor's stored values
	epoch      string          // the delta sync's epoch (items)
}

// Register: every route of the API — items, assets, renditions (of: the item's
// library), the perceptors the client is given (perceptors; values loads their
// stored values), the app's info
func Register(e *echo.Echo, db Store, of LibraryOf, cache string, info AppInfo, perceptors []api.Perceptor, values ValuesLoader, logger *l.Logger) {
	r := &routes{db: db, of: of, cache: cache, logger: logger, values: values}
	r.registerItems("/items", e)
	r.registerAssets("/assets", e)
	r.registerPerceptors(e, perceptors)
	r.registerRenditions(e)
	RegisterAppRoutes(e, info)
}
