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
func (p *sizePerceptor) Decorator(logger *l.Logger) chain.Decorator[exif_core.RawItemRW, exif_core.RawItemRW] {
	return &sizesExtractor{logger}
}

// ExifTags: the size pairs in their order, ImageSize, what turns it
func (p *sizePerceptor) ExifTags() []string {
	return append(append([]string{}, sizePairs...), "ImageSize", "Orientation", "Rotation")
}

var Perceptor api.Perceptor = &sizePerceptor{}
