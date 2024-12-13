package scan

import (
	"fmt"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/dukobpa3/perceplib/api"
	"github.com/dukobpa3/perceplib/chain"
	l "github.com/dukobpa3/perceplib/logger"
)

// Opner implementation
type opener struct {
	logger *l.Logger
}

func (fd *opener) Decorate(in *RawItem) (exif_core.RawItemRW, error) {
	if in.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	return in, nil
}

func (fd *opener) Stop() {}

func newOpener(chin <-chan *RawItem, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	processor := &opener{logger}
	return chain.NewDecorator(chin, chout, processor)
}

// Closer implementation
type closer struct {
	logger *l.Logger
}

func (fd *closer) Decorate(in exif_core.RawItemRW) (*dto.ItemDto, error) {
	if in == nil {
		return nil, chain.ErrSkippedItem
	}

	rawItem, ok := in.(*RawItem)
	if !ok {
		return nil, fmt.Errorf("expected *RawItem, got %T", in)
	}

	if rawItem.Item == nil {
		return nil, chain.ErrSkippedItem
	}

	return itemsProxy.UpdateItem(rawItem.Item)
}

func (fd *closer) Stop() {}

func newCloser(chin <-chan exif_core.RawItemRW, chout chan<- *dto.ItemDto, logger *l.Logger) chain.Processor {
	processor := &closer{logger}
	return chain.NewDecorator(chin, chout, processor)
}

// Chain implementation
func NewExifPluginProcessor(chin <-chan *RawItem, chout chan<- *dto.ItemDto, errch chan error, logger *l.Logger) chain.Processor {
	if itemsProxy == nil {
		itemsProxy = model.NewProxy(logger.Named("DB"))
	}

	exifChain := chain.NewChainProcessor(errch)

	exifPlugins := make([]api.Perceptor, 0)
	for _, p := range plugins.Pm.GetPlugins() {
		if p.DataProvider() == api.ExifDataProvider {
			exifPlugins = append(exifPlugins, p)
		}
	}

	channels := make([]chan exif_core.RawItemRW, len(exifPlugins)+1)
	for i := range channels {
		channels[i] = make(chan exif_core.RawItemRW, 1)
	}

	exifChain.AddStep(newOpener(chin, channels[0], logger.Named("opener")))

	processors := make([]chain.Processor, len(exifPlugins))
	for i, plugin := range exifPlugins {
		if exifPlugin, ok := plugin.(exif_core.ExifCorePerceptor); ok {
			processors[i] = exifPlugin.NewProcessor(channels[i], channels[i+1], logger.Named(plugin.Name()))
			exifChain.AddStep(processors[i])
		}
	}

	exifChain.AddStep(newCloser(channels[len(channels)-1], chout, logger.Named("closer")))

	return exifChain
}
