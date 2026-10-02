package transcode

import (
	"perceptrail/gontroller/pkg/importer/flow"

	"github.com/eggs-gd/perceplib/chain"
)

// Package transcode routes an asset (the whole group) to the transcoder of its
// kind, one sub-package per transcoder (photo, video, livephoto). The transcoders
// are stubs until thumbnails exist: they pass the asset on to the plugins.

const (
	BranchPhoto = iota
	BranchVideo
	BranchLivePhoto
)

type Switch struct{}

// NewSwitch: every asset to the transcoder of its kind
func NewSwitch(chin <-chan *flow.RawItem, toPhoto, toVideo, toLivePhoto chan<- *flow.RawItem) chain.Processor {
	return chain.NewSwitch(chin, []chan<- *flow.RawItem{BranchPhoto: toPhoto, BranchVideo: toVideo, BranchLivePhoto: toLivePhoto}, Switch{})
}

func (Switch) Switch(it *flow.RawItem) (map[int]*flow.RawItem, error) {
	switch {
	case it.Kinds[0] == flow.KindVideo && it.HasKind(flow.KindImage): // the video with its photo
		return map[int]*flow.RawItem{BranchLivePhoto: it}, nil
	case it.Kinds[0] == flow.KindVideo:
		return map[int]*flow.RawItem{BranchVideo: it}, nil
	default: // an image, or RAW (maybe with its JPEG: a ready preview later)
		return map[int]*flow.RawItem{BranchPhoto: it}, nil
	}
}

func (Switch) Stop() {}
