package exif

import (
	"gontroller/ext/chain"
	"gontroller/ext/exiftool"
	t "gontroller/pkg/_t"
	"log"
)

type exifExtractor struct {
	workers []*exiftool.Server
	freeCh  chan *exiftool.Server
}

func (cd *exifExtractor) getWorker() *exiftool.Server {
	return <-cd.freeCh
}

func (cd *exifExtractor) releaseWorker(worker *exiftool.Server) {
	cd.freeCh <- worker
}

func (cd *exifExtractor) Decorate(in t.ItemPath) t.RawExif {
	args := append(genericTags, []string{string(in)}...)
	log.Printf("ETM.Process.Command -> args: %v", args)

	et := cd.getWorker()
	defer cd.releaseWorker(et)
	out, err := et.Command(args...)
	if err != nil {
		log.Printf("ETM.Process.Command -> Stdout err: %v\n", err)
	}

	res := make(map[string][]byte)
	err = exiftool.Unmarshal(out, res)
	if err == nil {
		res["path"] = []byte(in)
		return t.RawExif(res)
	}

	return t.RawExif{}
}

func (cd *exifExtractor) Close() {
	for _, et := range cd.workers {
		et.Close()
	}
}

func NewExifExtractor(count int, chin <-chan t.ItemPath, chout chan<- t.RawExif) chain.Processor {
	workers := make([]*exiftool.Server, count)
	freeCh := make(chan *exiftool.Server, count)
	for i := 0; i < count; i++ {
		var et, err = exiftool.NewServer(commonArgs...)
		log.Printf("ETM.NewWorker -> file: %v, et: %v", et, err)
		if err != nil {
			log.Printf("ETM.NewWorker.panic -> et: %v, err: %v", et, err)
			panic(err)
		}
		workers[i] = et
		freeCh <- et
	}

	processor := &exifExtractor{}

	return chain.NewDecorator(chin, chout, processor)
}
