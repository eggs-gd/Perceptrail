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

// Enter: Path ->
// - WalkDir: Path -> ItemEntry - just dummy scan without filtering and logic
// - groups validator: ItemEntry -> []ItemEntry - merge separate files to groups, check if known/new/changed, write group to DB (Files Table) with updated links to each other and last scan date
// Exit: -> []ItemEntry - set New/Dirty/Deleted state for Item in DB ()

// Enter: []ItemEntry ->
// - exiftool: []ItemEntry -> []RawExif - just dummy extracting exifdata for each file of group. Main file - whole bunch, sidecars only needed
// - metadata processor: []RawExif -> RawExif - updating metadata for main file including data from sidecars
// - gate keeper: RawExif -> model.ItemDto - check by hash, output only really new
// Exit -> model.ItemDto, write updated metadata to DB (Items Table), set Processing state for Item in DB ()

// Enter: model.ItemDto ->
// - photo/video splitter: model.ItemDto -> model.ItemDto - photo/video splitted into 2 channels
// - photo processor: model.ItemDto(photo) -> ??? - transcoding from raw formats into webp, scaling to couple thumbnails sises
// - video processor: model.ItemDto(video) -> ??? - transcoding from raw formats into x264/x265, scaling(?, generating gif preview?)
// Exit ???? - set Ready state for Item to db (means that all needed files created and stored in formatted folders)

type importerService struct {
	appCtx app.AppContext

	errch chan error
	files chan []*dto.FileDto
	items chan *dto.ItemDto

	importChain chain.ChainProcessor
}

func NewImporterService(ctx app.AppContext) *importerService {
	logger := ctx.Logger(string(app.LogImporter))

	itemsProxy = model.NewProxy(ctx.Logger(string(app.LogDB)))
	filesProxy = model.NewProxy(ctx.Logger(string(app.LogDB)))

	var errch chan error = make(chan error)

	var files chan []*dto.FileDto = make(chan []*dto.FileDto)
	var rawItems chan *RawItem = make(chan *RawItem)
	var items chan *dto.ItemDto = make(chan *dto.ItemDto, 1000)

	// var photos chan model.ItemDto = make(chan model.ItemDto)
	// var videos chan model.ItemDto = make(chan model.ItemDto)

	go func() {
		for err := range errch {
			if errors.Is(err, chain.ErrSkippedItem) {
				logger.Info("Import Error", l.Error(err))
			} else {
				logger.Error("Import Error", l.Error(err))
			}

		}
	}()

	importChain := chain.NewChainProcessor(errch)
	importChain.AddStep(NewFsWalker(ctx.Config().Path, files, logger))
	importChain.AddStep(NewExifExtractor(5, files, rawItems, logger))
	importChain.AddStep(NewExifPluginProcessor(rawItems, items, errch, logger))

	//importChain.AddStep(NewTranscoder(5, items, items))

	return &importerService{
		appCtx:      ctx,
		errch:       errch,
		files:       files,
		items:       items,
		importChain: importChain,
	}
}

func (s *importerService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	s.importChain.Process(ctx)

	<-ctx.Done()
}
