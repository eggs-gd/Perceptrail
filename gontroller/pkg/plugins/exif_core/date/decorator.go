package date

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

type datesExtractor struct {
	logger *l.Logger
}

func (cd *datesExtractor) Decorate(in exif_core.RawItemRW) (exif_core.RawItemRW, error) {
	if d, ok := resolveDate(in); ok {
		in.SetDateInfo(d.time, d.source, d.zone)
	}
	return in, nil
}

func (cd *datesExtractor) Stop() {}

func NewDatesProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, &datesExtractor{logger})
}
