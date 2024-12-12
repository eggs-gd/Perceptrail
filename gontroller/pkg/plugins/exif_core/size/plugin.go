package size

import (
	"perceptrail/api"
	"perceptrail/chain"
	"perceptrail/gontroller/pkg/plugins/exif_core"
	l "perceptrail/logger"
)

type sizePerceptor struct{}

func (p *sizePerceptor) Name() string                       { return "exif_size" }
func (p *sizePerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *sizePerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *sizePerceptor) NewProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return NewSizesProcessor(chin, chout, logger)
}

func NewPerceptor() api.Perceptor {
	return &sizePerceptor{}
}
