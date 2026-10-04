package duration

import (
	"perceptrail/gontroller/internal/perceptor/builtin"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type durationPerceptor struct{}

func (p *durationPerceptor) Name() string                       { return "exif_duration" }
func (p *durationPerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *durationPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *durationPerceptor) Decorator(logger *l.Logger) chain.Decorator[builtin.Item, builtin.Item] {
	return &durationExtractor{logger}
}

// ExifTags: the length tags
func (p *durationPerceptor) ExifTags() []string { return durationTags }

var Perceptor api.Perceptor = &durationPerceptor{}
