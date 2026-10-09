package identify

import (
	"image"
	"image/color"
	"testing"
)

func TestAverageColor(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 200, G: 80, B: 40, A: 255})
	img.Set(1, 0, color.RGBA{R: 100, G: 40, B: 20, A: 255})

	if got, want := averageColor(img), "#963c1e"; got != want {
		t.Fatalf("averageColor() = %q, want %q", got, want)
	}
}

func TestPreviewColorFallsBackForRemoteImage(t *testing.T) {
	if got := previewColor("guid-1", "immich://guid/preview.jpg", "image/jpeg", nil); got == "" {
		t.Fatal("previewColor() returned no fallback for remote image")
	}
}

func TestPreviewColorFallsBackForVideo(t *testing.T) {
	if got := previewColor("guid-1", "/media/video.mov", "video/quicktime", nil); got == "" {
		t.Fatal("previewColor() returned no fallback for video")
	}
}
