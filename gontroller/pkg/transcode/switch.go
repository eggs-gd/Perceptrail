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
func NewSwitch(in *chain.Pipe[*Item], toPhoto, toVideo, toLivePhoto *chain.Pipe[*Item]) chain.Processor {
	return chain.Route(in, []*chain.Pipe[*Item]{BranchPhoto: toPhoto, BranchVideo: toVideo, BranchLivePhoto: toLivePhoto}, Switch{})
}

// Route: the branch of the asset's kind
func (Switch) Route(it *Item) (int, error) {
	switch it.kind() {
	case dto.KindLive: // the video with its photo
		return BranchLivePhoto, nil
	case dto.KindVideo:
		return BranchVideo, nil
	default: // an image, or RAW (maybe with its JPEG: a ready preview later)
		return BranchPhoto, nil
	}
}
