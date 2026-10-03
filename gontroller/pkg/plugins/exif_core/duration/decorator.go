package duration

import (
	"strconv"
	"strings"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// The length of a video (or an animation), for the tile: seconds (exiftool -n; the
// Apple Photos record writes them the same way).
// durationTags: where a length is, the best first
var durationTags = []string{"Duration", "MediaDuration", "TrackDuration"}

type durationExtractor struct {
	logger *l.Logger
}

func (d *durationExtractor) Decorate(in exif_core.RawItemRW) (exif_core.RawItemRW, error) {
	for _, tag := range durationTags {
		if s := parse(in.GetExif(tag)); s > 0 {
			in.SetDuration(s)
			break
		}
	}
	return in, nil
}

// parse: seconds, 0 if none
func parse(v string) float64 {
	s, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || s < 0 {
		return 0
	}
	return s
}

func (d *durationExtractor) Stop() {}

func NewDurationProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, &durationExtractor{logger})
}
