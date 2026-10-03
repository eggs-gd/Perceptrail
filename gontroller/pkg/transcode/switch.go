package transcode

import (
	"perceptrail/gontroller/pkg/model/dto"

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
func NewSwitch(chin <-chan *Item, toPhoto, toVideo, toLivePhoto chan<- *Item) chain.Processor {
	return chain.NewSwitch(chin, []chan<- *Item{BranchPhoto: toPhoto, BranchVideo: toVideo, BranchLivePhoto: toLivePhoto}, Switch{})
}

func (Switch) Switch(it *Item) (map[int]*Item, error) {
	switch it.kind() {
	case dto.KindLive: // the video with its photo
		return map[int]*Item{BranchLivePhoto: it}, nil
	case dto.KindVideo:
		return map[int]*Item{BranchVideo: it}, nil
	default: // an image, or RAW (maybe with its JPEG: a ready preview later)
		return map[int]*Item{BranchPhoto: it}, nil
	}
}

func (Switch) Stop() {}
