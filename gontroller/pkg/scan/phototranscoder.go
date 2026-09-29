package scan

import "github.com/eggs-gd/perceplib/chain"

type photoTranscoder struct{}

// NewPhotoTranscoder: thumbnails of an image or RAW asset (a JPEG next to a RAW
// can serve as a ready preview). Not implemented: passes the item on.
func NewPhotoTranscoder(chin <-chan *RawItem, chout chan<- *RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, photoTranscoder{})
}

func (photoTranscoder) Decorate(it *RawItem) (*RawItem, error) { return it, nil }
func (photoTranscoder) Stop()                                  {}
