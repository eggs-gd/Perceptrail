package identify

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// Sizes: images from their header, the main file from the source's metadata
func TestSetSizes(t *testing.T) {
	dir := t.TempDir()
	still := filepath.Join(dir, "s.jpg")
	out, err := os.Create(still)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(out, image.NewRGBA(image.Rect(0, 0, 64, 48)), nil); err != nil {
		t.Fatal(err)
	}
	out.Close()

	it := &draft{
		Files: []*dto.FileDto{
			{Role: dto.RoleOriginal, ItemEntry: dto.ItemEntry{Path: filepath.Join(dir, "o.heic")}},
			{Role: dto.RoleStill, ItemEntry: dto.ItemEntry{Path: still}},
		},
		Exif: []api.RawExif{{"ImageWidth": []byte("4032"), "ImageHeight": []byte("3024")}, nil},
		Meta: api.RawExif{"ImageWidth": []byte("3024"), "ImageHeight": []byte("4032")},
	}
	setSizes(it)
	if f := it.Files[0]; f.Width != 3024 || f.Height != 4032 {
		t.Errorf("main %dx%d, want the source's 3024x4032", f.Width, f.Height)
	}
	if f := it.Files[1]; f.Width != 64 || f.Height != 48 {
		t.Errorf("still %dx%d, want 64x48 from the header", f.Width, f.Height)
	}
}

// A derivative standing in for a cloud-only original has its own size, not the
// original's from the metadata
func TestSetSizesDerivativeAsMain(t *testing.T) {
	dir := t.TempDir()
	still := filepath.Join(dir, "d.jpg")
	out, _ := os.Create(still)
	jpeg.Encode(out, image.NewRGBA(image.Rect(0, 0, 32, 24)), nil)
	out.Close()
	it := &draft{
		Files: []*dto.FileDto{{Role: dto.RoleStill, ItemEntry: dto.ItemEntry{Path: still}}},
		Exif:  []api.RawExif{nil},
		Meta:  api.RawExif{"ImageWidth": []byte("4032"), "ImageHeight": []byte("3024")},
	}
	setSizes(it)
	if f := it.Files[0]; f.Width != 32 || f.Height != 24 {
		t.Errorf("%dx%d, want the derivative's 32x24", f.Width, f.Height)
	}
}
