package identify

import (
	"sync"
	"time"

	"github.com/eggs-gd/go-exiftool"

	l "github.com/eggs-gd/perceplib/logger"
)

// One file must not block an exiftool worker forever (broken or huge files)
const exiftoolTimeout = 2 * time.Minute

var commonArgs []string = []string{}

// exiftoolPool: long-lived exiftool processes (-stay_open) shared by the steps
// that need exiftool (exif, cheap preview). A command takes a free process.
type exiftoolPool struct {
	workers   []*exiftool.Server
	free      chan *exiftool.Server
	closeOnce sync.Once
}

func newExiftoolPool(count int, logger *l.Logger) *exiftoolPool {
	p := &exiftoolPool{free: make(chan *exiftool.Server, count)}
	for range count {
		et, err := exiftool.NewServer(commonArgs...)
		if err != nil {
			logger.Panic("exiftool: can't start", l.Error(err))
		}
		et.SetTimeout(exiftoolTimeout)
		p.workers = append(p.workers, et)
		p.free <- et
	}
	return p
}

func (p *exiftoolPool) Command(args ...string) ([]byte, error) {
	et := <-p.free
	defer func() { p.free <- et }()
	return et.Command(args...)
}

// Close: every step that uses the pool calls it on stop; the processes close once
func (p *exiftoolPool) Close() {
	p.closeOnce.Do(func() {
		for _, et := range p.workers {
			et.Close()
		}
	})
}
