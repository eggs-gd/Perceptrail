package scan

import (
	"context"
	"errors"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/scan/flow"
	"perceptrail/gontroller/pkg/scan/groups"
	"perceptrail/gontroller/pkg/scan/groups/apple"
	"perceptrail/gontroller/pkg/scan/groups/generic"
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
// - source switch -> groupers (generic | apple): files -> whole assets (FileGroup)
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
// - Apple Photos is off: its library goes to generic, which reads only originals/
//   (a derivative must never become an item).
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

	// fswalker -> source switch: one file (path + stat), or the end-of-walk marker
	files := make(chan flow.FileEvent)
	// source switch -> its grouper: the same, split by source; the marker goes to both
	toGeneric, toApple := make(chan flow.FileEvent), make(chan flow.FileEvent)
	// groupers -> files gate: a complete group (no main file yet), and/or the
	// grouper's marker; both groupers write here
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

	// Files -> whole assets, a grouper per source
	importChain.AddStep(groups.NewSourceSwitch(files, toGeneric, toApple))
	importChain.AddStep(generic.NewGrouper(toGeneric, grouped))
	importChain.AddStep(apple.NewGrouper(toApple, grouped))

	// Assets -> items: only what needs work, then metadata, kinds, identity
	importChain.AddStep(NewFilesGate(groups.Branches, progress, grouped, stored, logger))

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
	}
}

func (s *importerService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Nothing consumes finished items yet (later: events to the client); drain them,
	// or the closer blocks once the buffer is full
	go func() {
		for {
			select {
			case <-s.items:
				s.progress.finished()
			case <-ctx.Done():
				return
			}
		}
	}()

	s.importChain.Process(ctx)

	<-ctx.Done()
}
