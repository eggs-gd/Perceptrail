package scan

import (
	"gontroller/ext/chain"
	t "gontroller/pkg/_t"
	"gontroller/pkg/exif"
	"gontroller/pkg/fswatcher"
	"gontroller/pkg/model"
	"gontroller/pkg/validator"
	"sync"
)

var importChain *chain.Chain

func Run() {
	var wg *sync.WaitGroup

	var files chan t.ItemPath = make(chan t.ItemPath)
	var rawexifs chan exif.RawExif = make(chan exif.RawExif)

	var items chan model.ItemDto = make(chan model.ItemDto)

	var photos chan model.ItemDto = make(chan model.ItemDto)
	var videos chan model.ItemDto = make(chan model.ItemDto)

	importChain = &chain.Chain{}

	// start point
	fileWalker := fswatcher.NewMonitor("", files, wg)

	// exif extractor ItemPath -> RawExif
	// validator: RawExif -> RawExif (only new)
	// decorator: RawExif -> model.ItemDto
	// photo/video splitter: model.ItemDto -> photo/video splitted

	importChain.AddStep(exif.NewExifExtractor(5, files, rawexifs))
	importChain.AddStep(validator.NewValidator(5, rawexifs, items))

	wg.Add(1)
	go importChain.Process()

	wg.Add(1)
	go fileWalker.Walk()
}
