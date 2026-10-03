// Package video: poster, downscaled previews, a motion preview on mouseover, a
// browser-playable video. Not implemented: passes the asset on.
package video

import (
	"perceptrail/gontroller/pkg/transcode"

	"github.com/eggs-gd/perceplib/chain"
)

type Transcoder struct{}

func NewTranscoder(chin <-chan *transcode.Item, chout chan<- *transcode.Item) chain.Processor {
	return chain.NewDecorator(chin, chout, Transcoder{})
}

func (Transcoder) Decorate(it *transcode.Item) (*transcode.Item, error) { return it, nil }
func (Transcoder) Stop()                                                {}
