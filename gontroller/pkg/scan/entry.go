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
	"perceptrail/gontroller/pkg/scan/transcode"
	"perceptrail/gontroller/pkg/scan/transcode/livephoto"
	"perceptrail/gontroller/pkg/scan/transcode/photo"
	"perceptrail/gontroller/pkg/scan/transcode/video"

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
// - files gate: only new / changed / not Ready groups go on; deletions after the walk
// - exif: exiftool for every file of the group, in parallel
// - mime: the kind of every file; the main file is the source (RAW > video > image)
// - validator: the item — same / changed / moved (keeps its GUID) / new
// Exit: -> RawItem (the item with its whole group)
//
// Enter: RawItem ->
// - transcode switch (photo | video | Live Photo): outputs for the whole asset
// - plugins (date, size, perceptors), closer: the item is Ready
// Exit: -> ItemDto
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

// exiftool processes and parallel exif steps
const exifWorkers = 5

type importerService struct {
	appCtx app.AppContext

	errch chan error
	items chan *dto.ItemDto

	importChain chain.ChainProcessor
}

func NewImporterService(ctx app.AppContext) *importerService {
	logger := ctx.Logger(string(app.LogImporter))

	itemsProxy = model.NewProxy(ctx.Logger(string(app.LogDB)))
	filesProxy = model.NewProxy(ctx.Logger(string(app.LogDB)))

	errch := make(chan error)
	go func() {
		for err := range errch {
			if errors.Is(err, chain.ErrSkippedItem) {
				continue // dropped on purpose (unchanged, not media, marker consumed)
			}
			logger.Error("Import Error", l.Error(err))
		}
	}()

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
	// validator -> transcode switch: + Item (the GUID); not media and moved-and-done
	// items do not get here
	validated := make(chan *flow.RawItem)
	// transcode switch -> its transcoder: the same, split by the kind of the asset
	toPhoto, toVideo, toLivePhoto := make(chan *flow.RawItem), make(chan *flow.RawItem), make(chan *flow.RawItem)
	// transcoders -> plugins: the item with its outputs (stubs: passed on as is);
	// all transcoders write here
	transcoded := make(chan *flow.RawItem)
	// closer -> nobody yet: finished items, drained in Start (later: events to the
	// client); buffered so the closer does not wait for the drain
	items := make(chan *dto.ItemDto, 1000)

	importChain := chain.NewChainProcessor(errch)
	// Find every file under the library root
	importChain.AddStep(NewFsWalker(ctx.Config().Path, files, logger))

	// Files -> whole assets, a grouper per source
	importChain.AddStep(groups.NewSourceSwitch(files, toGeneric, toApple))
	importChain.AddStep(generic.NewGrouper(toGeneric, grouped))
	importChain.AddStep(apple.NewGrouper(toApple, grouped))

	// Assets -> items: only what needs work, then metadata, kinds, identity
	importChain.AddStep(NewFilesGate(groups.Branches, grouped, stored, logger))
	importChain.AddStep(NewExifExtractor(exifWorkers, stored, exifed, errch, logger))
	importChain.AddStep(NewMimeRanker(exifed, ranked))
	importChain.AddStep(NewValidator(ranked, validated, logger))

	// Outputs (thumbnails, previews) per kind of asset
	importChain.AddStep(transcode.NewSwitch(validated, toPhoto, toVideo, toLivePhoto))
	importChain.AddStep(photo.NewTranscoder(toPhoto, transcoded))
	importChain.AddStep(video.NewTranscoder(toVideo, transcoded))
	importChain.AddStep(livephoto.NewTranscoder(toLivePhoto, transcoded))

	// Metadata plugins and perceptors, then the item is Ready
	importChain.AddStep(NewExifPluginProcessor(transcoded, items, errch, logger))

	return &importerService{
		appCtx:      ctx,
		errch:       errch,
		items:       items,
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
			case <-ctx.Done():
				return
			}
		}
	}()

	s.importChain.Process(ctx)

	<-ctx.Done()
}
