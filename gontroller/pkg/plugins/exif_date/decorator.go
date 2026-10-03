package exif_date

import (
	"perceptrail/gontroller/pkg/plugins"

	l "github.com/eggs-gd/perceplib/logger"
)

type datesExtractor struct {
	logger *l.Logger
}

func (cd *datesExtractor) Decorate(in plugins.RawItemRW) (plugins.RawItemRW, error) {
	if d, ok := resolveDate(in); ok {
		in.SetDateInfo(d.time, d.source, d.zone)
	}
	return in, nil
}
