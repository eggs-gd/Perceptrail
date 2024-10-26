package importer

import (
	"gontroller/ext/chain"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
	"log"
	"sync"
)

var importChain *chain.Chain

func Chain(path string, wg *sync.WaitGroup) {

	var errch chan error = make(chan error)

	var files chan t.ItemPath = make(chan t.ItemPath)
	var rawexifs chan t.RawExif = make(chan t.RawExif)

	var items chan model.ItemDto = make(chan model.ItemDto)

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
	fileWalker := NewFsMonitor(path, files, wg)

	// exif extractor ItemPath -> RawExif
	// gate keeper: RawExif -> model.ItemDto (only new)
	// photo/video splitter: model.ItemDto -> photo/video splitted

	importChain.AddStep(NewExifExtractor(5, files, rawexifs))
	importChain.AddStep(NewGatekeeper(5, rawexifs, items))

	wg.Add(1)
	go importChain.Process()

	wg.Add(1)
	go fileWalker.Walk()

	wg.Wait()
}
