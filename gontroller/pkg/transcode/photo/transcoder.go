// Package photo: thumbnails of an image or RAW asset (a JPEG next to a RAW can
// serve as a ready preview). Not implemented: passes the asset on.
package photo

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
