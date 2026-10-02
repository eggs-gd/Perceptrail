// Package photo: thumbnails of an image or RAW asset (a JPEG next to a RAW can
// serve as a ready preview). Not implemented: passes the asset on.
package photo

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
