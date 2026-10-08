package render

import (
	"strings"
	"testing"
)

// The filters: HDR is tone mapped from its own tagged colour when ffmpeg can, never
// guessed, and then tagged SDR; without zscale its HDR tags stay; the size only
// shrinks
func TestFilters(t *testing.T) {
	full, plain := &ffmpeg{tonemap: hdrToSDR}, &ffmpeg{}
	hlg := stream{w: 3840, h: 2160, transfer: "arib-std-b67"}
	sdr := stream{w: 640, h: 360}

	if got := full.filters(hlg, 1280); !strings.HasPrefix(got, "zscale=tin=arib-std-b67:pin=bt2020:min=bt2020nc:") ||
		!strings.Contains(got, "scale=1280:1280:force_original_aspect_ratio=decrease") || !strings.HasSuffix(got, sdrTags) {
		t.Errorf("HLG with zscale: %s", got)
	}
	if got := plain.filters(hlg, 1280); strings.Contains(got, "zscale") || strings.Contains(got, sdrTags) {
		t.Errorf("HLG without zscale: %s — its own HDR tags must stay, not SDR's", got)
	}
	if got := full.filters(sdr, 1280); strings.Contains(got, "zscale") || !strings.HasPrefix(got, "scale=trunc(iw/2)*2") {
		t.Errorf("a small SDR video: %s", got)
	}
}

// The video is the first video stream that is no cover: a Movie Maker WMV lists its
// screenshot first (an attached picture, as ffprobe -show_entries gives it)
func TestProbedSkipsCover(t *testing.T) {
	wmv := `{"streams": [
	  {"index": 0, "width": 640, "height": 480, "disposition": {"attached_pic": 1}},
	  {"index": 1, "width": 640, "height": 480, "color_transfer": "unknown", "disposition": {"attached_pic": 0}}
	], "format": {"duration": "206.540000"}}`
	in, err := probed([]byte(wmv))
	if err != nil || in.index != 1 || in.w != 640 || in.duration.Seconds() != 206.54 || in.transfer != "" {
		t.Errorf("got %+v, %v: stream 1, its size, the length", in, err)
	}
	if _, err := probed([]byte(`{"streams": [{"index": 0, "width": 640, "height": 480, "disposition": {"attached_pic": 1}}]}`)); err == nil {
		t.Error("a cover alone taken for a video")
	}
}
