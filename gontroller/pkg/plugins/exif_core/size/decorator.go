package size

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"
	"strconv"

	"github.com/dukobpa3/perceplib/api"
	"github.com/dukobpa3/perceplib/chain"

	l "github.com/dukobpa3/perceplib/logger"
)

type sizesExtractor struct {
	logger *l.Logger
}

func (cd *sizesExtractor) Decorate(in exif_core.RawItemRW) (exif_core.RawItemRW, error) {
	widthStr := in.GetExif("ImageWidth")
	heightStr := in.GetExif("ImageHeight")

	w, _ := strconv.Atoi(string(widthStr))
	h, _ := strconv.Atoi(string(heightStr))

	size := in.GetSize()
	if size.W == w && size.H == h {
		return in, nil
	}

	in.SetSize(api.Size{W: w, H: h})

	in.SetRatio(api.GetRatio(in.GetSize()))

	return in, nil
}

func (cd *sizesExtractor) Stop() {}

func NewSizesProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	processor := &sizesExtractor{logger}

	return chain.NewDecorator(chin, chout, processor)
}
