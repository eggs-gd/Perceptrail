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

// The import chain, one small step per node: _sb/puml/Import chain.puml
//
//	fswalker -> groups switch (generic | Apple Photos) -> files gate -> exif (N) ->
//	mime -> validator -> transcode switch (photo | video | Live Photo) -> plugins -> closer

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
	toGroupers := []chan fileEvent{branchGeneric: make(chan fileEvent), branchPhotos: make(chan fileEvent)}
	groups := make(chan fileGroup)
	stored := make(chan storedGroup)
	exifed := make(chan exifGroup)
	ranked := make(chan exifGroup)
	validated := make(chan *RawItem)
	toTranscoders := []chan *RawItem{branchPhoto: make(chan *RawItem), branchVideo: make(chan *RawItem), branchLivePhoto: make(chan *RawItem)}
	transcoded := make(chan *RawItem)
	items := make(chan *dto.ItemDto, 1000)

	importChain := chain.NewChainProcessor(errch)
	importChain.AddStep(NewFsWalker(ctx.Config().Path, files, logger))

	importChain.AddStep(chain.NewSwitch(files, sendOnly(toGroupers), sourceSwitch{}))
	importChain.AddStep(chain.NewDecorator(toGroupers[branchGeneric], groups, newGenericGrouper()))
	importChain.AddStep(chain.NewDecorator(toGroupers[branchPhotos], groups, photosGrouper{}))

	importChain.AddStep(chain.NewDecorator(groups, stored, newFilesGate(groupBranches, logger)))
	for _, step := range NewExifExtractor(exifWorkers, stored, exifed, logger) {
		importChain.AddStep(step)
	}
	importChain.AddStep(chain.NewDecorator(exifed, ranked, mimeStep{}))
	importChain.AddStep(chain.NewDecorator(ranked, validated, newValidator(logger)))

	importChain.AddStep(chain.NewSwitch(validated, sendOnly(toTranscoders), transcodeSwitch{}))
	for _, in := range toTranscoders {
		importChain.AddStep(chain.NewDecorator(in, transcoded, transcodeStub{}))
	}

	importChain.AddStep(NewExifPluginProcessor(transcoded, items, errch, logger))

	return &importerService{
		appCtx:      ctx,
		errch:       errch,
		items:       items,
		importChain: importChain,
	}
}

// sendOnly: the switch takes its branches as send-only channels
func sendOnly[T any](chs []chan T) []chan<- T {
	out := make([]chan<- T, len(chs))
	for i, ch := range chs {
		out[i] = ch
	}
	return out
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
