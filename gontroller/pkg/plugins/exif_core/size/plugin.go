package size

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type sizePerceptor struct{}

func (p *sizePerceptor) Name() string                       { return "exif_size" }
func (p *sizePerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *sizePerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *sizePerceptor) NewProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return NewSizesProcessor(chin, chout, logger)
}

// ExifTags: the size pairs in their order, ImageSize, what turns it
func (p *sizePerceptor) ExifTags() []string {
	return append(append([]string{}, sizePairs...), "ImageSize", "Orientation", "Rotation")
}

var Perceptor api.Perceptor = &sizePerceptor{}
