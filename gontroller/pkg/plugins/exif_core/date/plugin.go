package date

import (
	"perceptrail/api"
	"perceptrail/chain"
	"perceptrail/gontroller/pkg/plugins/exif_core"
	l "perceptrail/logger"
)

type datePerceptor struct{}

func (p *datePerceptor) Name() string                       { return "exif_date" }
func (p *datePerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *datePerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *datePerceptor) NewProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return NewDatesProcessor(chin, chout, logger)
}

func NewPerceptor() api.Perceptor {
	return &datePerceptor{}
}
