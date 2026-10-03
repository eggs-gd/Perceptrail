// Package livephoto: the whole asset — the video (source) with its photo:
// thumbnails and a motion preview. Not implemented: passes the asset on.
package livephoto

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
