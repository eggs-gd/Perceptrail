package scan

// transcode: a switch by the kind of the main file. The branches are stubs until
// the transcoder exists: they pass the item on to the plugins.

const (
	branchPhoto = iota
	branchVideo
	branchLivePhoto
	transcodeBranches
)

type transcodeSwitch struct{}

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
