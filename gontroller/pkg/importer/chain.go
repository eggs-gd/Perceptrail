package importer

import (
	"gontroller/ext/chain"
	"gontroller/pkg/model/dto"
	"log"
	"sync"
)

var importChain *chain.Chain

func Chain(path string, wg *sync.WaitGroup) {

	var errch chan error = make(chan error)

	var files chan []dto.FileDto = make(chan []dto.FileDto)
	//var rawexifs chan t.RawExif = make(chan t.RawExif)

	var items chan dto.ItemDto = make(chan dto.ItemDto, 1000)

	// var photos chan model.ItemDto = make(chan model.ItemDto)
	// var videos chan model.ItemDto = make(chan model.ItemDto)

	go func() {
		for err := range errch {
			log.Printf("Error from Import Chain: %v", err)
		}
	}()

	importChain = chain.NewChainProcessor(errch, wg)
	defer importChain.Close()

	// start point
	fileWalker := NewFsWalker(path, files, wg)

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

	importChain.AddStep(NewMetaProcessor(5, files, items))
	//importChain.AddStep(NewGatekeeper(5, rawexifs, items))

	wg.Add(1)
	go importChain.Process()

	wg.Add(1)
	go fileWalker.Walk()

	wg.Wait()
}
