package scan

import (
	"context"
	"errors"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

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
// - fswalker (fswalker.go): root -> fileEvent - reports every file it finds (path +
//   stat), nothing else. An unreadable subdirectory is skipped and recorded; the
//   walk ends with a marker carrying the walk result (complete? unreadable dirs?).
// - groups switch (groups.go): fileEvent -> fileEvent - routes a file to the grouper
//   of its source; the marker goes to every grouper. Apple Photos is off
//   (photosLibraryEnabled): the library goes to generic for now.
// - groupers (groups.go): fileEvent -> fileGroup - a buffer of open groups inside;
//   a group goes out when it is complete (not ranked yet: no main file). generic:
//   sidecars by name, next to each other, so one open group; the last one goes out
//   with the marker. Apple Photos (stub): will group by the library's DB.
// Exit: -> fileGroup - files that belong together
//
// Enter: fileGroup ->
// - files gate (gate.go): fileGroup -> storedGroup - the files table: finds/creates
//   the rows, refreshes stat, stamps CheckTime. Lets through only groups that need
//   work: new or changed files, never linked, the item missing or not Ready (New,
//   Dirty, interrupted); the rest is dropped, so unchanged files never reach
//   exiftool. After the marker from every grouper: deletions — files not stamped
//   by this walk are gone (only after a complete walk that found files, never under
//   an unreadable dir): main file -> item Deleted, sidecar -> item Dirty.
// - exif (exifextractor.go): storedGroup -> exifGroup - exiftool -all for every file
//   of the group (the main file is unknown yet); N steps in parallel on the same
//   channels, groups are independent from here on.
// - mime (mime.go): exifGroup -> exifGroup - the kind of every file (exif MIMEType ->
//   own extension table -> content sniff; no system MIME tables), then the rank:
//   the main file is the source — RAW > video > image; the JPEG of RAW+JPEG and
//   the photo of a Live Photo are derivatives (sidecars). Nothing to show -> not media.
// - validator (validator.go): exifGroup -> RawItem - not media: rows marked ignored,
//   dropped. Otherwise links the files to the main file (a former main file that is
//   a sidecar now loses its item), then by the short hash of the main file (size +
//   non-volatile exif): same / changed (Dirty) / moved (the item keeps its GUID,
//   also restored if deletions ran first) / new or duplicate (a new item). A moved
//   item with complete outputs is Ready at once: no transcode, no plugins.
// Exit: -> RawItem - the item (GUID) with its whole group, main file first
//
// Enter: RawItem ->
// - transcode switch (transcoder.go): by the kind of the asset: photo (image, RAW)
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

	files := make(chan fileEvent)
	toGeneric, toPhotos := make(chan fileEvent), make(chan fileEvent)
	groups := make(chan fileGroup)
	stored := make(chan storedGroup)
	exifed := make(chan exifGroup)
	ranked := make(chan exifGroup)
	validated := make(chan *RawItem)
	toPhoto, toVideo, toLivePhoto := make(chan *RawItem), make(chan *RawItem), make(chan *RawItem)
	transcoded := make(chan *RawItem)
	items := make(chan *dto.ItemDto, 1000)

	importChain := chain.NewChainProcessor(errch)
	importChain.AddStep(NewFsWalker(ctx.Config().Path, files, logger))

	importChain.AddStep(NewSourceSwitch(files, toGeneric, toPhotos))
	importChain.AddStep(NewGenericGrouper(toGeneric, groups))
	importChain.AddStep(NewPhotosGrouper(toPhotos, groups))

	importChain.AddStep(NewFilesGate(groupBranches, groups, stored, logger))
	importChain.AddStep(NewExifExtractor(exifWorkers, stored, exifed, errch, logger))
	importChain.AddStep(NewMimeRanker(exifed, ranked))
	importChain.AddStep(NewValidator(ranked, validated, logger))

	importChain.AddStep(NewTranscodeSwitch(validated, toPhoto, toVideo, toLivePhoto))
	importChain.AddStep(NewPhotoTranscoder(toPhoto, transcoded))
	importChain.AddStep(NewVideoTranscoder(toVideo, transcoded))
	importChain.AddStep(NewLivePhotoTranscoder(toLivePhoto, transcoded))

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
