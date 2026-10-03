package identify

import (
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/importer/discover"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

const sidecarXMP = `<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?>
<x:xmpmeta xmlns:x='adobe:ns:meta/'><rdf:RDF xmlns:rdf='http://www.w3.org/1999/02/22-rdf-syntax-ns#'>
<rdf:Description rdf:about='' xmlns:exif='http://ns.adobe.com/exif/1.0/'>
<exif:DateTimeOriginal>2024-05-06T07:08:09</exif:DateTimeOriginal>
</rdf:Description></rdf:RDF></x:xmpmeta><?xpacket end='w'?>`

// One exiftool call reads the whole group: every file its own map, only the
// declared tags (and identify's own), values as exiftool prints them; a file it
// cannot read gets none
func TestReadGroup(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not installed")
	}
	dir := t.TempDir()
	photo := filepath.Join(dir, "a.jpg")
	f, err := os.Create(photo)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, image.NewGray(image.Rect(0, 0, 64, 48)), nil); err != nil {
		t.Fatal(err)
	}
	f.Close()
	// South and west, a portrait orientation: what -n gives for them is pinned here
	if out, err := exec.Command("exiftool", "-q", "-overwrite_original",
		"-GPSLatitude=33.8688", "-GPSLatitudeRef=S", "-GPSLongitude=151.2093", "-GPSLongitudeRef=W",
		"-Orientation#=6", photo).CombinedOutput(); err != nil {
		t.Fatalf("exiftool write: %v %s", err, out)
	}
	xmp := filepath.Join(dir, "a.xmp")
	if err := os.WriteFile(xmp, []byte(sidecarXMP), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone.jpg")

	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	pool := newExiftoolPool(1, logger)
	defer pool.Close()
	r := NewReader(pool, []string{"DateTimeOriginal", "GPSLatitude", "GPSLongitude", "Orientation"}, logger)

	file := func(p string) *dto.FileDto { return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: p}} }
	d, err := r.Decorate(discover.Group{Files: []*dto.FileDto{file(photo), file(xmp), file(gone)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(d.Exif[0]["ImageWidth"]); got != "64" {
		t.Errorf("ImageWidth %q, want 64", got)
	}
	// -n: numbers as numbers — the composite GPS tags signed by their Ref, the
	// orientation 1–8, ImageSize "W H"
	for tag, want := range map[string]string{
		"GPSLatitude": "-33.8688", "GPSLongitude": "-151.2093", "Orientation": "6", "ImageSize": "64 48",
	} {
		if got := string(d.Exif[0][tag]); got != want {
			t.Errorf("%s %q, want %q", tag, got, want)
		}
	}
	if got := string(d.Exif[0]["MIMEType"]); got != "image/jpeg" {
		t.Errorf("MIMEType %q", got)
	}
	if _, ok := d.Exif[0]["FileSize"]; ok {
		t.Error("an undeclared tag was read")
	}
	if _, ok := d.Exif[0]["SourceFile"]; ok {
		t.Error("SourceFile left in the map")
	}
	if got := string(d.Exif[1]["DateTimeOriginal"]); got != "2024:05:06 07:08:09" {
		t.Errorf("the sidecar's DateTimeOriginal %q", got)
	}
	if d.Exif[2] != nil {
		t.Errorf("a missing file has metadata: %v", d.Exif[2])
	}
}
