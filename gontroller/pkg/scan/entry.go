package scan

import (
	"context"
	"errors"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/scan/flow"
	"perceptrail/gontroller/pkg/scan/groups"
	"sync"
	"time"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

var filesProxy model.FilesApi
var itemsProxy model.ItemsApi

// The import chain: one step per node, steps talk over channels.
// Diagrams: _sb/puml/Import chain.puml, _sb/puml/Walker.puml (gate, validator).
//
// Enter: library root ->
// - fswalker: every file found (path + stat), then the end-of-walk marker
// - switch -> the grouper of the first provider that claims the file
//   (providers.Enabled: Apple Photos…, the plain folder last): files -> whole assets
//   (FileGroup)
// Exit: -> FileGroup
//
// Enter: FileGroup ->
// - files gate: only new / changed / unfinished groups go on; deletions after the walk
// - exif: exiftool for every file of the group, in parallel
// - mime: the kind of every file; the main file is the source (RAW > video > image)
// - validator: the item — same / changed / moved (keeps its GUID) / new
// Exit: -> RawItem (the item with its whole group)
//
// Enter: RawItem ->
// - cheap preview: what the browser can show right now (the original, a derivative,
//   an embedded preview) — no transcode
// - plugins (date, size, perceptors), closer: Visible (a preview) or Waiting (none)
// Exit: -> ItemDto
//
// Later, a chain of its own (roadmap: thumbnails): transcode switch (photo | video |
// Live Photo) fed from the DB -> Ready. Its packages (transcode/…) are not wired yet.
//
// Not obvious:
// - The end-of-walk marker goes through the groupers (they flush their last
//   group): deletions run only when every grouper's marker reached the gate.
// - Deletions only after a complete walk that found files, never under an
//   unreadable directory: an unmounted drive must not wipe the library.
// - The chain is async: deletions may run before a moved file is validated, so the
//   validator restores deleted items by hash.
// - A provider not enabled is not in the chain: its files are a plain folder's.
//   One that is groups its files its own way (Apple: by the library's DB, the groups
//   formed up front; a group that could not complete is held back, its files are
//   not "gone"); its key is the item's GUID, whatever the main file.
// - The walk repeats: rescan after the last group of the previous walk is done
//   (not after the walk — processing takes longer), so walks never overlap.

// exiftool processes and parallel exif steps
const exifWorkers = 5

// Pause between the end of one walk's processing and the next walk (config: rescan)
const defaultRescan = time.Minute

type importerService struct {
	appCtx app.AppContext

	errch    chan error
	items    chan *dto.ItemDto
	progress *progress

	importChain chain.ChainProcessor

	// One asset again (Refresh): its provider forms its group, the gate takes it like
	// any other; whoever waits hears when its item leaves the chain or the gate drops
	// the group
	grouped chan flow.FileGroup
	waits   *assetWaits
}

func NewImporterService(ctx app.AppContext) *importerService {
	logger := ctx.Logger(string(app.LogImporter))

	itemsProxy = model.NewProxy(ctx.Logger(string(app.LogDB)))
	filesProxy = model.NewProxy(ctx.Logger(string(app.LogDB)))
	if err := reclassifyIgnored(model.NewProxy(ctx.Logger(string(app.LogDB))), filesProxy, logger); err != nil {
		logger.Error("MIME version check failed", l.Error(err))
	}

	logErr := func(err error) {
		if !errors.Is(err, chain.ErrSkippedItem) { // skips are on purpose (buffered, unchanged, not media)
			logger.Error("Import Error", l.Error(err))
		}
	}
	progress := newProgress()

	// Walk, groups, gate
	errch := make(chan error)
	go func() {
		for err := range errch {
			logErr(err)
		}
	}()
	// After the gate every group ends as an item (items) or here: both count as done
	errProcessing := make(chan error)
	go func() {
		for err := range errProcessing {
			progress.finished()
			logErr(err)
		}
	}()
	rescan := ctx.Config().Rescan
	if rescan <= 0 {
		rescan = defaultRescan
	}

	// Channels between the steps: from -> to, what it carries. The message types
	// are in the flow package (FileEvent, FileGroup, WalkResult, RawItem).

	// fswalker -> switch: one file (path + stat), or the end-of-walk marker. The
	// switch -> the grouper of the first provider that claims the file (the plain
	// folder claims the rest); the marker to every grouper.
	files := make(chan flow.FileEvent)
	// groupers -> files gate: a complete group (no main file yet), and/or the
	// grouper's marker; every grouper writes here
	grouped := make(chan flow.FileGroup)
	// files gate -> exif: the same group, stored: rows of the files table (GUIDs);
	// only groups that need work
	stored := make(chan flow.FileGroup)
	// exif -> mime: RawItem with Files + Exif
	exifed := make(chan *flow.RawItem)
	// mime -> validator: + Kinds, the main file first
	ranked := make(chan *flow.RawItem)
	// validator -> cheap preview: + Item (the GUID); not media does not get here
	validated := make(chan *flow.RawItem)
	// cheap preview -> plugins: + Item.PreviewPath/PreviewMime ("" = nothing yet)
	previewed := make(chan *flow.RawItem)
	// closer -> nobody yet: finished items, drained in Start (later: events to the
	// client); buffered so the closer does not wait for the drain
	items := make(chan *dto.ItemDto, 1000)

	importChain := chain.NewChainProcessor(errch)
	// Find every file under the library root
	importChain.AddStep(NewFsWalker(ctx.Config().Path, rescan, progress, files, logger))

	// Files -> whole assets: a file to the grouper of the first provider that claims
	// it (Apple Photos…, the plain folder last)
	ps := providers.Enabled()
	toGroupers := make([]chan<- flow.FileEvent, len(ps))
	for i, p := range ps {
		toGrouper := make(chan flow.FileEvent)
		toGroupers[i] = toGrouper
		importChain.AddStep(chain.NewDecorator(toGrouper, grouped, p.Grouper()))
	}
	importChain.AddStep(groups.NewSwitch(ps, files, toGroupers))

	// Assets -> items: only what needs work, then metadata, kinds, identity. A keyed
	// group it drops is told to whoever waits for that asset (Refresh)
	waits := newAssetWaits()
	importChain.AddStep(NewFilesGate(len(ps), progress, waits.done, grouped, stored, logger))

	// The rest reports to errProcessing: progress counts the groups in flight
	processing := chain.NewChainProcessor(errProcessing)
	exiftool := newExiftoolPool(exifWorkers, logger)
	processing.AddStep(NewExifExtractor(exifWorkers, exiftool, stored, exifed, errProcessing, logger))
	processing.AddStep(NewMimeRanker(exifed, ranked))
	processing.AddStep(NewValidator(ranked, validated, logger))

	// Show what exists (no transcode), then metadata plugins and perceptors; the
	// closer: Visible or Waiting
	processing.AddStep(NewCheapPreview(exiftool, ctx.Config().CacheDir(), validated, previewed, logger))
	processing.AddStep(NewExifPluginProcessor(previewed, items, errProcessing, logger))

	// After its steps: AddStep hands the sub-chain the outer error channel, its
	// steps keep errProcessing
	importChain.AddStep(processing)

	return &importerService{
		appCtx:      ctx,
		errch:       errch,
		items:       items,
		progress:    progress,
		importChain: importChain,
		grouped:     grouped,
		waits:       waits,
	}
}

// Refresh processes one asset of a provider's library again, now — the library (Apple
// Photos…) has just made a file of it local (on demand): no walk of the whole library
// for one photo. Its group goes to the gate like any group (only what changed passes; deletions are not
// touched — they come with the walk's marker; a walk sending the same asset at the
// same time just processes it twice into the same item). Waits until the item has
// left the chain or the gate dropped the group (nothing changed: Photos drew from
// what was local), at most wait; false if neither (no such asset, too slow) — the
// next walk catches up anyway.
func (s *importerService) Refresh(uuid string, wait time.Duration) bool {
	var group flow.FileGroup
	ok := false
	for _, p := range providers.Enabled() {
		if group, ok = p.Regroup(uuid); ok {
			break
		}
	}
	if !ok {
		return false
	}
	done := s.waits.add(uuid)
	defer s.waits.drop(uuid, done)

	timeout := time.After(wait)
	select {
	case s.grouped <- group:
	case <-timeout:
		return false
	}
	select {
	case <-done:
		return true
	case <-timeout:
		return false
	}
}

// assetWaits: who waits for an asset's item to be done (Refresh) — told when the
// item leaves the chain or the gate drops its group
type assetWaits struct {
	mu sync.Mutex
	by map[string][]chan struct{}
}

func newAssetWaits() *assetWaits { return &assetWaits{by: map[string][]chan struct{}{}} }

// add: a channel closed when the asset is done
func (w *assetWaits) add(key string) chan struct{} {
	ch := make(chan struct{})
	w.mu.Lock()
	w.by[key] = append(w.by[key], ch)
	w.mu.Unlock()
	return ch
}

// drop: the waiter gave up (or heard already)
func (w *assetWaits) drop(key string, ch chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	chs := w.by[key]
	for i, c := range chs {
		if c == ch {
			w.by[key] = append(chs[:i], chs[i+1:]...)
			break
		}
	}
	if len(w.by[key]) == 0 {
		delete(w.by, key)
	}
}

// done: the asset's item is done — everyone waiting hears
func (w *assetWaits) done(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.by[key] {
		close(c)
	}
	delete(w.by, key)
}

func (s *importerService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Nothing consumes finished items yet (later: events to the client); drain them,
	// or the closer blocks once the buffer is full
	go func() {
		for {
			select {
			case it := <-s.items:
				s.progress.finished()
				s.waits.done(it.Guid)
			case <-ctx.Done():
				return
			}
		}
	}()

	s.importChain.Process(ctx)

	<-ctx.Done()
}
