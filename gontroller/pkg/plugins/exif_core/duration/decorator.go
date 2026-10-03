package duration

import (
	"strconv"
	"strings"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"
)

// The length of a video (or an animation), for the tile. exiftool prints it as
// "24.40 s" under 30 s, "0:01:23" above (without -n); the Apple Photos record
// writes it the same way.
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

// parse: seconds from "24.40 s", "0:01:23", "1:02:03.5", "24.4"; 0 if none
func parse(v string) float64 {
	v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "(approx)"))
	v = strings.TrimSpace(strings.TrimSuffix(v, "s"))
	var total float64
	for _, part := range strings.Split(v, ":") {
		n, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || n < 0 {
			return 0
		}
		total = total*60 + n
	}
	return total
}

func (d *durationExtractor) Stop() {}

func NewDurationProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, &durationExtractor{logger})
}
