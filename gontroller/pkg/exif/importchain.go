package exif

import (
	"gontroller/ext/chain"
	"gontroller/ext/exiftool"
	t "gontroller/pkg/_t"
	"log"
)

type RawExif map[string][]byte

type exifExtractor struct {
	workers []*exiftool.Server
}

func (cd *exifExtractor) getWorker() *exiftool.Server {
	return cd.workers[0] // todo: track busy and check free, return first free if it is
}

func (cd *exifExtractor) Decorate(in t.ItemPath) RawExif {
	args := append(genericTags, []string{string(in)}...)
	log.Printf("ETM.Process.Command -> args: %v", args)

	et := cd.getWorker()
	out, err := et.Command(args...)
	if err != nil {
		log.Printf("ETM.Process.Command -> Stdout err: %v\n", err)
	}

	res := make(map[string][]byte)
	err = exiftool.Unmarshal(out, res)
	if err == nil {
		res["path"] = []byte(in)
		return RawExif(res)
	}

	return RawExif{}
}

func NewExifExtractor(count int, chin <-chan t.ItemPath, chout chan<- RawExif) chain.ControlBase {
	var workers = make([]*exiftool.Server, count)
	for i := 0; i < count; i++ {
		var et, err = exiftool.NewServer(commonArgs...)
		log.Printf("ETM.NewWorker -> file: %v, et: %v", et, err)
		if err != nil {
			log.Printf("ETM.NewWorker.panic -> et: %v, err: %v", et, err)
			panic(err)
		}
		workers[i] = et
	}

	processor := &exifExtractor{}

	return chain.NewDecorator(chin, chout, processor)

}
