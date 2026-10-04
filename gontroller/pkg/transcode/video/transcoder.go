// Package video: poster, downscaled previews, a motion preview on mouseover, a
// browser-playable video. Not implemented: passes the asset on.
package video

import (
	"perceptrail/gontroller/pkg/transcode"

	"github.com/eggs-gd/perceplib/chain"
)

type Transcoder struct{}

func NewTranscoder(in <-chan *transcode.Item, out chan<- *transcode.Item) chain.Processor {
	return chain.NewDecorator(in, out, Transcoder{})
}

func (Transcoder) Decorate(it *transcode.Item) (*transcode.Item, error) { return it, nil }
