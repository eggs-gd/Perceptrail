package chain

import (
	t "gontroller/pkg/_t"
	exf "gontroller/pkg/exifreader"
	fsw "gontroller/pkg/fswatcher"

	"encoding/json"
	"log"
	"strings"
	"sync"
)

// ItemPath -> ExifInfo -> ItemInfo {path, exifSize, realSize, []thumb}

type Chain struct {
	path string

	fileChan chan t.ItemPath
	exifChan chan t.ItemExif

	photoChan chan t.ItemExif
	videoChan chan t.ItemExif
	//var itemChan chan t.ItemInfo = make(chan t.ItemInfo)

	fsm *fsw.Monitor
	etm *exf.Monitor

	wg *sync.WaitGroup
}

func NewChain(path string) *Chain {
	return &Chain{
		path: path,

		fileChan: make(chan t.ItemPath),
		exifChan: make(chan t.ItemExif),

		photoChan: make(chan t.ItemExif, 1000),
		videoChan: make(chan t.ItemExif, 1000),
		//var itemChan chan t.ItemInfo = make(chan t.ItemInfo)
	}
}

func (ch *Chain) Run(wg *sync.WaitGroup) {
	ch.wg = wg

	ch.fsm = fsw.NewMonitor(ch.path, ch.fileChan, wg)
	ch.etm = exf.NewMonitorPool(10, ch.fileChan, ch.exifChan, wg)

	wg.Add(1)
	go func() {
		ch.fsm.Walk()

		wg.Add(1)
		ch.fsm.Watch()

		wg.Done()
	}()

	wg.Add(1)
	go func() {
		ch.etm.Process()
		wg.Done()
		//tsugor.DecorateAsync(10, fileChan, exifChan, etm.Decorator, wg)
	}()

	wg.Add(1)
	go func() {
		mimeFilter(ch.exifChan, ch.photoChan, ch.videoChan)
		wg.Done()
	}()

	wg.Wait()
}

func (ch *Chain) Close() {
	//ch.wg.Done()
	close(ch.fileChan)
	close(ch.exifChan)

	close(ch.photoChan)
	close(ch.videoChan)

	//var itemChan chan t.ItemInfo = make(chan t.ItemInfo)

	ch.fsm.Close()
	ch.etm.Close()
}

func mimeFilter(ex <-chan t.ItemExif, photos chan<- t.ItemExif, videos chan<- t.ItemExif) {
	for it := range ex {

		file, err := json.MarshalIndent(it, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling JSON: %v\n", err)
		}

		log.Printf("ItemExif -> %v", string(file))
		if strings.Contains(it.MimeType, "video/") {
			videos <- it
		} else if strings.Contains(it.MimeType, "image/") {
			photos <- it
		} else {
			//ignore ?
			//add to known list?
		}
	}
}
