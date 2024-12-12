package scan

import (
	"perceptrail/api"
	"perceptrail/chain"
	"perceptrail/exiftool"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	l "perceptrail/logger"
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

type exifExtractor struct {
	logger  *l.Logger
	workers []*exiftool.Server
	freeCh  chan *exiftool.Server
}

func (cd *exifExtractor) Decorate(in []*dto.FileDto) (*RawItem, error) {
	var result []api.RawExif

	for i, item := range in {
		var args []string
		if i == 0 {
			args = append(mainTags, item.Path)
		} else {
			args = append(metaTags, item.Path)
		}

		cd.logger.Info("Command", l.Any("args", args))

		et := cd.getWorker()
		defer cd.releaseWorker(et)

		out, err := et.Command(args...)
		if err != nil {
			cd.logger.Error("Command", l.Any("out", out), l.String("file", item.Path), l.Error(err))
		}

		res := map[string][]byte{}
		if err := exiftool.Unmarshal(out, res); err != nil {
			return &RawItem{}, err
		}

		if err == nil {
			result = append(result, api.RawExif(res))
		}
	}

	return cd.processMeta(in[0], result)
}

func (cd *exifExtractor) Stop() {
	close(cd.freeCh)

	for _, et := range cd.workers {
		et.Close()
	}
}

func NewExifExtractor(count int, chin <-chan []*dto.FileDto, chout chan<- *RawItem, logger *l.Logger) chain.Processor {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger.Named("DB"))
	}

	workers := make([]*exiftool.Server, count)
	freeCh := make(chan *exiftool.Server, count)
	for i := 0; i < count; i++ {
		var et, err = exiftool.NewServer(commonArgs...)
		logger.Info("NewWorker", l.Any("et", et), l.Error(err))
		if err != nil {
			logger.Panic("NewWorker", l.Any("et", et), l.Error(err))
		}
		workers[i] = et
		freeCh <- et
	}

	processor := &exifExtractor{logger, workers, freeCh}

	return chain.NewDecorator(chin, chout, processor)
}

func (cd *exifExtractor) getWorker() *exiftool.Server {
	return <-cd.freeCh
}

func (cd *exifExtractor) releaseWorker(worker *exiftool.Server) {
	cd.freeCh <- worker
}

func (cd *exifExtractor) processMeta(in *dto.FileDto, exifs []api.RawExif) (*RawItem, error) {
	res, err := itemsProxy.ValidateFile(in, exifs[0])
	if err != nil {
		return &RawItem{}, err
	}

	item := &RawItem{
		Exif: exifs,
		Item: res,
	}

	//todo ignore if updated?
	if res.State > dto.Dirty {
		//return nil, chain.ErrSkippedItem
	}

	return item, nil
}
