package importer

import (
	"gontroller/ext/chain"
	"gontroller/ext/exiftool"
	t "gontroller/pkg/_t"
	"log"
)

var commonArgs []string = []string{ // all sidecars
	"-srcfile",
	"@",
	"-filesize#",
}

var genericTags []string = []string{ // Generic tags needed for db.Item
	//"-FileType",
	"-MIMEType",
	"-ExifImageWidth",
	"-ExifImageHeight",
	"-ImageWidth",
	"-ImageHeight",
	// "-ThumbnailImageWidth",
	// "-ThumbnailImageHeight",
	// "-DisplayWidth",
	// "-DisplayHeight",
	// "-ImageSize",
	// "-SourceImageWidth",
	// "-SourceImageHeight",
	"-Duration",
	"-AvgBitrate",
	"-VideoCodec",
	"-AudioCodec",
}

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

func (cd *exifExtractor) Decorate(in t.ItemPath) (t.RawExif, error) {
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
		res[t.PerceptrailPathFieldName] = []byte(in)
		return t.RawExif(res), nil
	}

	return nil, nil
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
