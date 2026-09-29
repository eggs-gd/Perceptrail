// Package livephoto: the whole asset — the video (source) with its photo:
// thumbnails and a motion preview. Not implemented: passes the asset on.
package livephoto

import (
	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"
)

type Transcoder struct{}

func NewTranscoder(chin <-chan *flow.RawItem, chout chan<- *flow.RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, Transcoder{})
}

func (Transcoder) Decorate(it *flow.RawItem) (*flow.RawItem, error) { return it, nil }
func (Transcoder) Stop()                                            {}
