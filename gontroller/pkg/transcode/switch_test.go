package transcode

import (
	"testing"

	"perceptrail/gontroller/pkg/importer/flow"
)

func TestTranscodeSwitch(t *testing.T) {
	route := func(kinds ...flow.MediaKind) int {
		out, _ := Switch{}.Switch(&flow.RawItem{Kinds: kinds})
		for b := range out {
			return b
		}
		return -1
	}
	if route(flow.KindVideo, flow.KindImage) != BranchLivePhoto || route(flow.KindVideo) != BranchVideo ||
		route(flow.KindRaw, flow.KindImage) != BranchPhoto || route(flow.KindImage, flow.KindSidecar) != BranchPhoto {
		t.Error("wrong transcode branch")
	}
}
