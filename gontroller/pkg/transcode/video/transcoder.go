// Package video: poster, downscaled previews, a motion preview on mouseover, a
// browser-playable video. Not implemented: passes the asset on.
package video

import (
	"perceptrail/gontroller/pkg/importer/flow"

	"github.com/eggs-gd/perceplib/chain"
)

type Transcoder struct{}

func NewTranscoder(chin <-chan *flow.RawItem, chout chan<- *flow.RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, Transcoder{})
}

func (Transcoder) Decorate(it *flow.RawItem) (*flow.RawItem, error) { return it, nil }
func (Transcoder) Stop()                                            {}
