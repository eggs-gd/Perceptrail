package apple

import (
	"context"
	"sync"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
)

// Items: what the provider asks of the library's items — the ones nothing can show
// yet (on demand: asked of Photos), and the mark that has the next walk process one
// again (Photos made a file of it local)
type Items interface {
	Unshown() ([]*dto.ItemDto, error)
	MarkRework(guids []string) (int64, error)
}

// Provider: Apple Photos — the files of a *.photoslibrary grouped by its DB (the
// grouper), and on demand what Photos keeps only in iCloud (PhotoKit, macOS: see
// ondemand.go). We only read the library; Photos downloads into it.
type Provider struct {
	root    string // the library root walked: access to Photos is asked only if one is there
	grouper *Grouper
	photos  Photos // PhotoKit (macOS; elsewhere a stub that refuses)
	items   Items
	logger  *l.Logger

	// At most `fetchers` requests to Photos at once; one per asset and want, the
	// others wait for its result
	sem        chan struct{}
	inFlightMu sync.Mutex
	inFlight   map[string]*fetching
}

var _ provider.Provider = (*Provider)(nil)

// New: photos asks Photos for renditions (photokit.Library: PhotoKit on macOS, a
// stub that refuses elsewhere); root is the library root walked
func New(root string, photos Photos, items Items, logger *l.Logger) *Provider {
	return &Provider{
		root:     root,
		grouper:  newGrouper(logger),
		photos:   photos,
		items:    items,
		logger:   logger,
		sem:      make(chan struct{}, fetchers),
		inFlight: map[string]*fetching{},
	}
}

// Claims: a file inside a Photos library bundle
func (p *Provider) Claims(path string) bool { return BundleRoot(path) != "" }

func (p *Provider) Grouper() provider.Grouper { return p.grouper }

// Owns: an item whose main file is in a Photos library (its GUID is the asset UUID)
func (p *Provider) Owns(item *dto.ItemDto) bool { return BundleRoot(item.Path) != "" }

// Start: access to Photos (asked once — the prompt names the app that started us,
// the terminal), then the assets nothing shows yet, in the background
func (p *Provider) Start(ctx context.Context) {
	if !HasLibrary(p.root) {
		return
	}
	go func() {
		if !p.photos.Authorize() {
			return
		}
		p.logger.Info("Photos: renditions on demand")
		p.hydrateWaiting(ctx)
	}()
}

// refresh: Photos made a file of the asset local (or drew from what was local) —
// its item is processed again on the next walk, even if no file changed; the
// client guesses meanwhile
func (p *Provider) refresh(uuid string) {
	if _, err := p.items.MarkRework([]string{uuid}); err != nil {
		p.logger.Error("Photos: item not marked for the next walk", l.String("guid", uuid), l.Error(err))
	}
}
