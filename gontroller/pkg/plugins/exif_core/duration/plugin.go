package duration

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type durationPerceptor struct{}

func (p *durationPerceptor) Name() string                       { return "exif_duration" }
func (p *durationPerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *durationPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *durationPerceptor) NewProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return NewDurationProcessor(chin, chout, logger)
}

var Perceptor api.Perceptor = &durationPerceptor{}
