package apple

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
)

// Items: what the provider asks of the library's items — the ones nothing can show
// yet (on demand: asked of Photos), and the mark that has the next walk process one
// again (Photos made a file of it local)
type Items interface {
	Unshown() ([]*dto.ItemDto, error)
	MarkRework(guids []api.GUID) (int64, error)
}

// Provider: Apple Photos — the files of a *.photoslibrary grouped by its DB (the
// grouper), and on demand what Photos keeps only in iCloud (PhotoKit, macOS: see
// ondemand.go). We only read the library; Photos downloads into it.
type Provider struct {
	roots   []string // the roots walked: access to Photos is asked only if a library is in one
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
// stub that refuses elsewhere); roots are the library roots walked
func New(roots []string, photos Photos, items Items, logger *l.Logger) *Provider {
	return &Provider{
		roots:    roots,
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

// Skipped: in every Photos library of root, everything but the originals
// and Photos' renders and derivatives — its database (read from a copy, not
// walked), search index, caches, journals, the internal and private stores hold
// no asset's file
func (p *Provider) Skipped(root string) []string {
	var skipped []string
	for _, bundle := range bundles(root) {
		skipped = append(skipped, dirsBut(bundle, "originals", "resources")...)
		skipped = append(skipped, dirsBut(filepath.Join(bundle, "resources"), "renders", "derivatives")...)
	}
	return skipped
}

// Owns: an item whose main file is in a Photos library (its GUID is the asset UUID)
func (p *Provider) Owns(item *dto.ItemDto) bool { return BundleRoot(item.Path) != "" }

// File: none — its files are on the disk
func (p *Provider) File(string) http.Handler { return nil }

// Start: access to Photos (asked once — the prompt names the app that started us,
// the terminal), then the assets nothing shows yet, in the background
func (p *Provider) Start(ctx context.Context) {
	if !slices.ContainsFunc(p.roots, HasLibrary) {
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

// dirsBut: the directories in dir, but the kept ones
func dirsBut(dir string, kept ...string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if e.IsDir() && !slices.Contains(kept, e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// refresh: Photos made a file of the asset local (or drew from what was local) —
// its item is processed again on the next walk, even if no file changed; the
// client guesses meanwhile
func (p *Provider) refresh(guid api.GUID) {
	if _, err := p.items.MarkRework([]api.GUID{guid}); err != nil {
		p.logger.Error("Photos: item not marked for the next walk", l.String("guid", guid.String()), l.Error(err))
	}
}
