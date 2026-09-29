package scan

import "github.com/eggs-gd/perceplib/chain"

// transcode switch: routes the asset (the whole group) to the transcoder of its
// kind. The transcoders are stubs until thumbnails exist: they pass it on.

const (
	branchPhoto = iota
	branchVideo
	branchLivePhoto
	transcodeBranches
)

type transcodeSwitch struct{}

// NewTranscodeSwitch: every item to the transcoder of its kind
func NewTranscodeSwitch(chin <-chan *RawItem, toPhoto, toVideo, toLivePhoto chan<- *RawItem) chain.Processor {
	return chain.NewSwitch(chin, []chan<- *RawItem{branchPhoto: toPhoto, branchVideo: toVideo, branchLivePhoto: toLivePhoto}, transcodeSwitch{})
}

func (transcodeSwitch) Switch(it *RawItem) (map[int]*RawItem, error) {
	switch {
	case it.Kinds[0] == KindVideo && it.hasKind(KindImage): // the video with its photo
		return map[int]*RawItem{branchLivePhoto: it}, nil
	case it.Kinds[0] == KindVideo:
		return map[int]*RawItem{branchVideo: it}, nil
	default: // an image, or RAW (maybe with its JPEG: a ready preview later)
		return map[int]*RawItem{branchPhoto: it}, nil
	}
}

func (transcodeSwitch) Stop() {}
