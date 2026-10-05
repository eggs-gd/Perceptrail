// Package photo: thumbnails of an image or RAW asset (a JPEG next to a RAW can
// serve as a ready preview). Not implemented: passes the asset on.
package photo

import (
	"perceptrail/gontroller/internal/transcode"

	chain "github.com/eggs-gd/go-chain"
)

type Transcoder struct{}

func NewTranscoder(in <-chan *transcode.Item, out chan<- *transcode.Item) chain.Processor {
	return chain.NewDecorator(in, out, Transcoder{})
}

func (Transcoder) Decorate(it *transcode.Item) (*transcode.Item, error) { return it, nil }
