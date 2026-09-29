package date

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type datePerceptor struct{}

func (p *datePerceptor) Name() string                       { return "exif_date" }
func (p *datePerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *datePerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *datePerceptor) NewProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return NewDatesProcessor(chin, chout, logger)
}

var Perceptor api.Perceptor = &datePerceptor{}
