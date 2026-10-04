// Package livephoto: the whole asset — the video (source) with its photo:
// thumbnails and a motion preview. Not implemented: passes the asset on.
package livephoto

import (
	"perceptrail/gontroller/pkg/transcode"

	"github.com/eggs-gd/perceplib/chain"
)

type Transcoder struct{}

func NewTranscoder(in <-chan *transcode.Item, out chan<- *transcode.Item) chain.Processor {
	return chain.Decorate(in, out, Transcoder{})
}

func (Transcoder) Decorate(it *transcode.Item) (*transcode.Item, error) { return it, nil }
