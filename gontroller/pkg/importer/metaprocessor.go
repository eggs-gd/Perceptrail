package importer

import (
	"gontroller/ext/chain"
	"gontroller/ext/exiftool"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
	"gontroller/pkg/model/dto"
	"log"
)

var commonArgs []string = []string{ // all sidecars
	//"-j",
}

// exiftool -all --File:all --ExifToolVersion -s2 ./_D3A9906.JPG
var mainTags []string = []string{
	// todo: check with and without
	// "-a",
	"-all",
	"--File:all",
	"--ExifToolVersion",
	"-s2",
}

var metaTags []string = []string{ // Generic tags needed for db.Item
	//"-FileType",
	//"-MIMEType",
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

var validationProxy model.ValidationApi

type exifExtractor struct {
	workers []*exiftool.Server
	freeCh  chan *exiftool.Server
}

func (cd *exifExtractor) Decorate(in []*dto.FileDto) (*dto.ItemDto, error) {
	var result []t.RawExif

	for i, item := range in {
		var args []string
		if i == 0 {
			args = append(mainTags, item.Path)
		} else {
			args = append(metaTags, item.Path)
		}

		log.Printf("ETM.Process.Command -> args: %v", args)

		et := cd.getWorker()
		defer cd.releaseWorker(et)

		out, err := et.Command(args...)
		if err != nil {
			log.Printf("ETM.Process.Command -> Stdout err: %v\n", err)
		}

		res := map[string][]byte{}
		if err := exiftool.Unmarshal(out, res); err != nil {
			return &dto.ItemDto{}, err
		}

		if err == nil {
			result = append(result, t.RawExif(res))
		}
	}

	return cd.processMeta(in, result)
}

func (cd *exifExtractor) Stop() {
	close(cd.freeCh)

	for _, et := range cd.workers {
		et.Close()
	}
}

func NewMetaProcessor(count int, chin <-chan []*dto.FileDto, chout chan<- *dto.ItemDto) chain.Processor {
	if validationProxy == nil {
		validationProxy = model.NewProxy()
	}

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

	processor := &exifExtractor{workers, freeCh}

	return chain.NewDecorator(chin, chout, processor)
}

func (cd *exifExtractor) getWorker() *exiftool.Server {
	return <-cd.freeCh
}

func (cd *exifExtractor) releaseWorker(worker *exiftool.Server) {
	cd.freeCh <- worker
}

func (cd *exifExtractor) processMeta(in []*dto.FileDto, exifs []t.RawExif) (*dto.ItemDto, error) {
	res, err := validationProxy.ValidateFile(in[0], exifs[0])

	// todo fill available meta
	// todo check itemState
	/*
		type ItemDto struct {

		Date time.Time // CreationDate of asset

		Size  t.Size `gorm:"embedded;embeddedPrefix:size_"`
		Ratio t.Size `gorm:"embedded;embeddedPrefix:ratio_"`
		}
	*/

	if res.State > dto.Dirty {
		// ignore file
	}

	return res, err
}
