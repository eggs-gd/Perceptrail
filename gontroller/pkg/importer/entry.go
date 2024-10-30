package importer

import (
	"context"
	"gontroller/ext/chain"
	"gontroller/pkg/app"
	"gontroller/pkg/model/dto"
	"log"
)

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

type ImporterService struct {
	appCtx app.AppContext

	errch chan error
	files chan []*dto.FileDto
	items chan *dto.ItemDto

	importChain chain.ChainProcessor
}

func NewImporterService(ctx app.AppContext) *ImporterService {
	var errch chan error = make(chan error)

	var files chan []*dto.FileDto = make(chan []*dto.FileDto)
	var items chan *dto.ItemDto = make(chan *dto.ItemDto, 1000)

	// var photos chan model.ItemDto = make(chan model.ItemDto)
	// var videos chan model.ItemDto = make(chan model.ItemDto)

	go func() {
		for err := range errch {
			log.Printf("Error from Import Chain: %v", err)
		}
	}()

	importChain := chain.NewChainProcessor(errch)
	importChain.AddStep(NewFsWalker(ctx.Config().Path, files))
	importChain.AddStep(NewMetaProcessor(5, files, items))
	//importChain.AddStep(NewTranscoder(5, items, items))

	return &ImporterService{
		appCtx:      ctx,
		errch:       errch,
		files:       files,
		items:       items,
		importChain: importChain,
	}
}

func (s *ImporterService) Start(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	s.importChain.Process(ctx)

	<-ctx.Done()
}
