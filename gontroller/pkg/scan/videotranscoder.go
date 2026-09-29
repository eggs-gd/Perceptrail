package scan

import "github.com/eggs-gd/perceplib/chain"

type videoTranscoder struct{}

// NewVideoTranscoder: poster, downscaled previews, a motion preview on mouseover,
// a browser-playable video. Not implemented: passes the item on.
func NewVideoTranscoder(chin <-chan *RawItem, chout chan<- *RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, videoTranscoder{})
}

func (videoTranscoder) Decorate(it *RawItem) (*RawItem, error) { return it, nil }
func (videoTranscoder) Stop()                                  {}
