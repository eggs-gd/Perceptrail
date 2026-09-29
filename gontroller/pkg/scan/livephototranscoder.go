package scan

import "github.com/eggs-gd/perceplib/chain"

type livePhotoTranscoder struct{}

// NewLivePhotoTranscoder: the whole asset — the video (source) with its photo:
// thumbnails and a motion preview. Not implemented: passes the item on.
func NewLivePhotoTranscoder(chin <-chan *RawItem, chout chan<- *RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, livePhotoTranscoder{})
}

func (livePhotoTranscoder) Decorate(it *RawItem) (*RawItem, error) { return it, nil }
func (livePhotoTranscoder) Stop()                                  {}
