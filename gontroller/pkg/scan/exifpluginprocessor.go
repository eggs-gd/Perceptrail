package scan

import (
	"fmt"
	"perceptrail/gontroller/pkg/app"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/plugins"
	"perceptrail/gontroller/pkg/plugins/exif_core"
	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

// Opner implementation
type opener struct {
	logger *l.Logger
}

func (fd *opener) Decorate(in *flow.RawItem) (exif_core.RawItemRW, error) {
	if in.Item == nil {
		return nil, chain.ErrSkippedItem
	}
	return in, nil
}

func (fd *opener) Stop() {}

func newOpener(chin <-chan *flow.RawItem, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
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

	rawItem, ok := in.(*flow.RawItem)
	if !ok {
		return nil, fmt.Errorf("expected *flow.RawItem, got %T", in)
	}

	if rawItem.Item == nil {
		return nil, chain.ErrSkippedItem
	}

	// The end of the cheap stage: shown if there is a preview; Ready comes from the
	// expensive stage (transcode)
	rawItem.Item.State = dto.Waiting
	if rawItem.Item.PreviewPath != "" {
		rawItem.Item.State = dto.Visible
	}
	return itemsProxy.UpdateItem(rawItem.Item)
}

func (fd *closer) Stop() {}

func newCloser(chin <-chan exif_core.RawItemRW, chout chan<- *dto.ItemDto, logger *l.Logger) chain.Processor {
	processor := &closer{logger}
	return chain.NewDecorator(chin, chout, processor)
}

// Adapters for external EXIF plugins: they work with the read-only api.RawItemR,
// the core chain carries exif_core.RawItemRW
type toReadOnly struct{}

func (toReadOnly) Decorate(in exif_core.RawItemRW) (api.RawItemR, error) { return in, nil }
func (toReadOnly) Stop()                                                 {}

type toReadWrite struct{}

func (toReadWrite) Decorate(in api.RawItemR) (exif_core.RawItemRW, error) {
	rw, ok := in.(exif_core.RawItemRW)
	if !ok {
		return nil, fmt.Errorf("external EXIF plugin returned %T, want exif_core.RawItemRW", in)
	}
	return rw, nil
}
func (toReadWrite) Stop() {}

// Chain implementation
func NewExifPluginProcessor(chin <-chan *flow.RawItem, chout chan<- *dto.ItemDto, errch chan error, logger *l.Logger) chain.Processor {
	exifChain := chain.NewChainProcessor(errch)

	prev := make(chan exif_core.RawItemRW, 1)
	exifChain.AddStep(newOpener(chin, prev, logger.Named(string(app.LogPluginExifOpener))))

	// Every step reads the previous step's channel: a plugin that gets no step must
	// not get a channel either, otherwise the chain stalls on the unread one.
	for _, plugin := range plugins.Pm.GetPlugins() {
		if plugin.DataProvider() != api.ExifDataProvider {
			continue
		}
		pluginLogger := logger.Named(plugin.Name())

		switch p := plugin.(type) {
		case exif_core.ExifCorePerceptor:
			next := make(chan exif_core.RawItemRW, 1)
			proc := p.NewProcessor(prev, next, pluginLogger)
			if proc == nil {
				logger.Error("EXIF plugin returned no processor, skipped", l.String("plugin", plugin.Name()))
				continue
			}
			exifChain.AddStep(proc)
			prev = next

		case api.ExifPerceptor:
			in := make(chan api.RawItemR, 1)
			out := make(chan api.RawItemR, 1)
			proc := p.NewProcessor(in, out, pluginLogger)
			if proc == nil {
				logger.Error("EXIF plugin returned no processor, skipped", l.String("plugin", plugin.Name()))
				continue
			}
			next := make(chan exif_core.RawItemRW, 1)
			exifChain.AddStep(chain.NewDecorator[exif_core.RawItemRW, api.RawItemR](prev, in, toReadOnly{}))
			exifChain.AddStep(proc)
			exifChain.AddStep(chain.NewDecorator[api.RawItemR, exif_core.RawItemRW](out, next, toReadWrite{}))
			prev = next

		default:
			logger.Error("EXIF plugin has no NewProcessor, skipped", l.String("plugin", plugin.Name()))
		}
	}

	exifChain.AddStep(newCloser(prev, chout, logger.Named(string(app.LogPluginExifCloser))))

	return exifChain
}
