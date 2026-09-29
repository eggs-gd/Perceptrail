package scan

import "github.com/eggs-gd/perceplib/chain"

// transcode: a switch by the kind of the main file. The branches are stubs until
// the transcoder exists: they pass the item on to the plugins.

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

// transcodeStub: a branch of the transcoder that does nothing yet
type transcodeStub struct{}

func (transcodeStub) Decorate(it *RawItem) (*RawItem, error) { return it, nil }
func (transcodeStub) Stop()                                  {}

// NewPhotoTranscoder: thumbnails (not implemented: passes the item on)
func NewPhotoTranscoder(chin <-chan *RawItem, chout chan<- *RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, transcodeStub{})
}

// NewVideoTranscoder: poster, previews, a browser-playable video (not implemented)
func NewVideoTranscoder(chin <-chan *RawItem, chout chan<- *RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, transcodeStub{})
}

// NewLivePhotoTranscoder: the whole asset — photo and motion (not implemented)
func NewLivePhotoTranscoder(chin <-chan *RawItem, chout chan<- *RawItem) chain.Processor {
	return chain.NewDecorator(chin, chout, transcodeStub{})
}
