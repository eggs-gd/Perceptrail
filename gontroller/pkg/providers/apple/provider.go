package apple

import (
	"context"
	"sync"
	"time"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	l "github.com/eggs-gd/perceplib/logger"
)

// Items: what the provider reads of the library's items — the ones nothing can show
// yet (on demand: asked of Photos)
type Items interface {
	Unshown() ([]*dto.ItemDto, error)
}

// Provider: Apple Photos — the files of a *.photoslibrary grouped by its DB (the
// grouper), and on demand what Photos keeps only in iCloud (PhotoKit, macOS: see
// ondemand.go). We only read the library; Photos downloads into it.
type Provider struct {
	root    string // the library root walked: access to Photos is asked only if one is there
	grouper *Grouper
	photos  Photos // nil: only what is on disk
	items   Items
	logger  *l.Logger

	refresh providers.Refresher
	// At most `fetchers` requests to Photos at once; one per asset and want, the
	// others wait for its result
	sem        chan struct{}
	inFlightMu sync.Mutex
	inFlight   map[string]*fetching
}

var _ providers.Provider = (*Provider)(nil)

// New: photos asks Photos for renditions (photokit.Library; nil: only what is on
// disk); root is the library root walked
func New(root string, photos Photos, items Items, logger *l.Logger) *Provider {
	return &Provider{
		root:     root,
		grouper:  newGrouper(logger),
		photos:   photos,
		items:    items,
		logger:   logger,
		refresh:  func(string, time.Duration) bool { return false },
		sem:      make(chan struct{}, fetchers),
		inFlight: map[string]*fetching{},
	}
}

func (p *Provider) Name() string { return "apple" }

// Claims: a file inside a Photos library bundle
func (p *Provider) Claims(path string) bool { return BundleRoot(path) != "" }

func (p *Provider) Grouper() providers.Grouper { return p.grouper }

func (p *Provider) Regroup(key string) (providers.Asset, bool) { return p.grouper.Regroup(key) }

// Owns: an item whose main file is in a Photos library (its GUID is the asset UUID)
func (p *Provider) Owns(item *dto.ItemDto) bool { return BundleRoot(item.Path) != "" }

// Start: access to Photos (asked once — the prompt names the app that started us,
// the terminal), then the assets nothing shows yet, in the background
func (p *Provider) Start(ctx context.Context, refresh providers.Refresher) {
	p.refresh = refresh
	if p.photos == nil || !HasLibrary(p.root) {
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
