package date

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"

	l "github.com/eggs-gd/perceplib/logger"
)

type datesExtractor struct {
	logger *l.Logger
}

func (cd *datesExtractor) Decorate(in exif_core.RawItemRW) (exif_core.RawItemRW, error) {
	if d, ok := resolveDate(in); ok {
		in.SetDateInfo(d.time, d.source, d.zone)
	}
	return in, nil
}
