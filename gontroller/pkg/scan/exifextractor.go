package scan

import (
	"fmt"
	"slices"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/go-exiftool"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// One file must not block an exiftool worker forever (broken or huge files)
const exiftoolTimeout = 2 * time.Minute

var commonArgs []string = []string{ // all sidecars
	//"-j",
}

// exiftool -all --File:all --ExifToolVersion -s2 ./_D3A9906.JPG
var mainTags []string = []string{
	// todo: check with and without
	// "-a",
	"-all",
	//"--File:all",
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
		isMain := i == 0
		tags := metaTags
		if isMain {
			tags = mainTags
		}
		args := slices.Concat(tags, []string{item.Path})

		cd.logger.Info("Command", l.Any("args", args))

		et := cd.getWorker()
		out, cmdErr := et.Command(args...)
		cd.releaseWorker(et)

		res := map[string][]byte{}
		if len(out) > 0 {
			if err := exiftool.Unmarshal(out, res); err != nil && cmdErr == nil {
				cmdErr = err
			}
		}

		if len(res) == 0 {
			// Nothing usable. Without the main file's EXIF the group cannot become an item
			// (processMeta validates it against the first entry), a sidecar is just skipped.
			if cmdErr == nil {
				cmdErr = fmt.Errorf("no metadata")
			}
			if isMain {
				return nil, fmt.Errorf("exiftool %s: %w", item.Path, cmdErr)
			}
			cd.logger.Error("exiftool: sidecar skipped", l.String("file", item.Path), l.Error(cmdErr))
			continue
		}
		if cmdErr != nil {
			// ExifTool reported a problem but still returned data: use it
			cd.logger.Warn("exiftool reported a problem", l.String("file", item.Path), l.Error(cmdErr))
		}

		result = append(result, api.RawExif(res))
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
	workers := make([]*exiftool.Server, count)
	freeCh := make(chan *exiftool.Server, count)
	for i := 0; i < count; i++ {
		var et, err = exiftool.NewServer(commonArgs...)
		logger.Info("NewWorker", l.Any("et", et), l.Error(err))
		if err != nil {
			logger.Panic("NewWorker", l.Any("et", et), l.Error(err))
		}
		et.SetTimeout(exiftoolTimeout)
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
