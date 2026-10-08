package render

import (
	"strings"
	"testing"
)

// The filters: HDR is tone mapped from its own tagged colour when ffmpeg can, never
// guessed; SDR is not touched; the size only shrinks; the output is tagged SDR
func TestFilters(t *testing.T) {
	full, plain := &ffmpeg{tonemap: hdrToSDR}, &ffmpeg{}
	hlg := stream{w: 3840, h: 2160, transfer: "arib-std-b67"}
	sdr := stream{w: 640, h: 360}

	if got := full.filters(hlg, 1280); !strings.HasPrefix(got, "zscale=tin=arib-std-b67:pin=bt2020:min=bt2020nc:") ||
		!strings.Contains(got, "scale=1280:1280:force_original_aspect_ratio=decrease") || !strings.HasSuffix(got, sdrTags) {
		t.Errorf("HLG with zscale: %s", got)
	}
	if got := plain.filters(hlg, 1280); strings.Contains(got, "zscale") || !strings.HasSuffix(got, sdrTags) {
		t.Errorf("HLG without zscale: %s", got)
	}
	if got := full.filters(sdr, 1280); strings.Contains(got, "zscale") || !strings.HasPrefix(got, "scale=trunc(iw/2)*2") {
		t.Errorf("a small SDR video: %s", got)
	}
}
