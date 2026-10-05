package size

import (
	"perceptrail/gontroller/internal/perceptor/builtin"

	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
)

type sizePerceptor struct{}

func (p *sizePerceptor) Name() string                       { return "exif_size" }
func (p *sizePerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *sizePerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *sizePerceptor) Decorator(logger *l.Logger) chain.Decorator[builtin.Item, builtin.Item] {
	return &sizesExtractor{logger}
}

// ExifTags: the size pairs in their order, ImageSize, what turns it
func (p *sizePerceptor) ExifTags() []string {
	return append(append([]string{}, sizePairs...), "ImageSize", "Orientation", "Rotation")
}

var Perceptor api.Perceptor = &sizePerceptor{}
