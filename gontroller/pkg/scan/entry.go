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

// The import chain: one small step per node, every step is its own goroutine,
// every arrow is a channel. Diagrams: _sb/puml/Import chain.puml (the whole
// chain), _sb/puml/Walker.puml (files gate and validator in detail).
//
//	fswalker -> groups switch (generic | Apple Photos) -> files gate -> exif (N) ->
//	mime -> validator -> transcode switch (photo | video | Live Photo) -> plugins -> closer
//
// Enter: library root ->
// - fswalker (fswalker.go): root -> FileEvent - reports every file it finds (path +
//   stat), nothing else. An unreadable subdirectory is skipped and recorded; the
//   walk ends with a marker carrying the walk result (complete? unreadable dirs?).
// - source switch (groups/sourceswitch.go): FileEvent -> FileEvent - routes a file to the grouper
//   of its source; the marker goes to every grouper. Apple Photos is off
//   (groups.appleEnabled): the library goes to generic for now, which reads only
//   its originals/ — derivatives and Apple's own images must not become items.
// - groupers (groups/generic, groups/apple): FileEvent -> FileGroup - a buffer of open groups inside;
//   a group goes out when it is complete (not ranked yet: no main file). generic:
//   sidecars by name, next to each other, so one open group; the last one goes out
//   with the marker. Apple Photos (stub): will group by the library's DB.
// Exit: -> FileGroup - one whole asset: the files that belong together
//
// Enter: FileGroup ->
// - files gate (filesgate.go): FileGroup -> FileGroup - the files table: finds/creates
//   the rows, refreshes stat, stamps CheckTime. Lets through only groups that need
//   work: new or changed files, never linked, the item missing or not Ready (New,
//   Dirty, interrupted); the rest is dropped, so unchanged files never reach
//   exiftool. After the marker from every grouper: deletions — files not stamped
//   by this walk are gone (only after a complete walk that found files, never under
//   an unreadable dir): main file -> item Deleted, sidecar -> item Dirty.
// - exif (exifextractor.go): FileGroup -> RawItem - exiftool -all for every file of
//   the group (the main file is unknown yet): RawItem.Files + Exif, no Item yet; N
//   steps in parallel on the same channels, groups are independent from here on.
// - mime (mimeranker.go): RawItem -> RawItem - Kinds: the kind of every file (exif MIMEType ->
//   own extension table -> content sniff; no system MIME tables), then the rank:
//   the main file is the source — RAW > video > image; the JPEG of RAW+JPEG and
//   the photo of a Live Photo are derivatives (sidecars). Nothing to show -> not media.
// - validator (validator.go): RawItem -> RawItem - sets Item. Not media: rows marked ignored,
//   dropped. Otherwise links the files to the main file (a former main file that is
//   a sidecar now loses its item), then by the short hash of the main file (size +
//   non-volatile exif): same / changed (Dirty) / moved (the item keeps its GUID,
//   also restored if deletions ran first) / new or duplicate (a new item). A moved
//   item with complete outputs is Ready at once: no transcode, no plugins.
// Exit: -> RawItem - the item (GUID) with its whole group, main file first
//
// Enter: RawItem ->
// - transcode switch (transcode/switch.go; transcoders transcode/photo, video,
//   livephoto): by the kind of the asset: photo (image, RAW)
//   / video / Live Photo (a video with its photo). Transcoders take the whole group,
//   not a file. Stubs for now: they pass the item on.
// - plugins (exifpluginprocessor.go): core (date with its zone, size), then
//   external perceptors; after transcode, so the client gets items with thumbnails.
// - closer: saves the item, State Ready.
// Exit: -> ItemDto - finished items (drained for now; later: events to the client)

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
	importChain.AddStep(NewFsWalker(ctx.Config().Path, files, logger))

	importChain.AddStep(groups.NewSourceSwitch(files, toGeneric, toApple))
	importChain.AddStep(generic.NewGrouper(toGeneric, grouped))
	importChain.AddStep(apple.NewGrouper(toApple, grouped))

	importChain.AddStep(NewFilesGate(groups.Branches, grouped, stored, logger))
	importChain.AddStep(NewExifExtractor(exifWorkers, stored, exifed, errch, logger))
	importChain.AddStep(NewMimeRanker(exifed, ranked))
	importChain.AddStep(NewValidator(ranked, validated, logger))

	importChain.AddStep(transcode.NewSwitch(validated, toPhoto, toVideo, toLivePhoto))
	importChain.AddStep(photo.NewTranscoder(toPhoto, transcoded))
	importChain.AddStep(video.NewTranscoder(toVideo, transcoded))
	importChain.AddStep(livephoto.NewTranscoder(toLivePhoto, transcoded))

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
