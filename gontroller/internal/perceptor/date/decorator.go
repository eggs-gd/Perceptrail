package date

import (
	"perceptrail/gontroller/internal/perceptor/builtin"

	l "github.com/eggs-gd/perceplib/logger"
)

type datesExtractor struct {
	logger *l.Logger
}

func (cd *datesExtractor) Decorate(in builtin.Item) (builtin.Item, error) {
	if d, ok := resolveDate(in); ok {
		in.SetDateInfo(d.time, d.source, d.zone)
	}
	return in, nil
}
