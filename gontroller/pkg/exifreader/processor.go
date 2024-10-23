package exif

import (
	"errors"
	"gontroller/ext/exiftool"
	t "gontroller/pkg/_t"
	"log"
	"strconv"
	"strings"
	"sync"
)

var commonArgs []string = []string{
	"-srcfile",
	"@",
}

var genericTags []string = []string{
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

type Monitor struct {
	wg      *sync.WaitGroup
	workers []*exiftool.Server

	filesChan <-chan t.ItemPath
	infosChan chan<- t.ItemExif
}

func NewMonitorPool(count int, filesChan <-chan t.ItemPath, infosChan chan<- t.ItemExif, wg *sync.WaitGroup) *Monitor {
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

	m := &Monitor{
		wg: wg,

		workers: workers,

		filesChan: filesChan,
		infosChan: infosChan,
		//decorator: fromPathToExif,
	}

	return m
}

func (m *Monitor) Process() {
	defer func() {
		for _, et := range m.workers {
			log.Printf("ETM.Process.worker.shutdown -> et: %v", et)
			et.Shutdown()
		}
	}()

	defer m.wg.Done()

	//var err error
	//var out chan string = make(chan string)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for _, et := range m.workers {
			for file := range m.filesChan {
				args := append(genericTags, []string{string(file)}...)
				log.Printf("ETM.Process.Command -> args: %v", args)
				out, err := et.Command(args...)
				if err != nil {
					log.Printf("ETM.Process.Command -> Stdout err: %v\n", err)
				}

				res, err := unmarshall(string(out))
				if err == nil {
					res.Path = string(file)
					m.infosChan <- res
				}
			}
		}
		wg.Done()
	}()

	wg.Wait()

	// if err != nil {
	// 	log.Fatalf("Error Command running: %v\n", err)
	// 	panic(err)
	// }
}

func (m *Monitor) Close() {
	for _, et := range m.workers {
		log.Printf("ETM.Process.worker.shutdown -> et: %v", et)
		et.Close()
	}
}

func unmarshall(input string) (t.ItemExif, error) {
	log.Printf("ETM.unmarshal -> %v", input)
	strArr := strings.Split(input, "\n")

	var resMap = make(map[string]string)
	var path string

	for _, str := range strArr {
		if strings.Index(str, " ") == 0 {
			return t.ItemExif{}, errors.New("footer")
		} else if strings.Index(str, "=") == 0 {
			path = strings.Split(str, "= ")[1]
		} else if strings.Contains(str, ":") {
			splitted := strings.Split(str, ": ")
			resMap[strings.TrimSpace(splitted[0])] = splitted[1]
		} else {
			//return t.ItemExif{}, errors.New("empty string")
		}
	}

	var res = t.ItemExif{
		Path:     string(path),
		MimeType: resMap["MIME Type"],
	}

	if val, err := strconv.ParseInt(resMap["Image Width"], 0, 64); err == nil {
		res.Size.W = int(val)
	}
	if val, err := strconv.ParseInt(resMap["Image Height"], 0, 64); err == nil {
		res.Size.H = int(val)
	}

	if val, err := strconv.ParseInt(resMap["Exif Image Width"], 0, 64); err == nil {
		res.ExifSize.W = int(val)
	}
	if val, err := strconv.ParseInt(resMap["Exif Image Height"], 0, 64); err == nil {
		res.ExifSize.H = int(val)
	}

	return res, nil
}
