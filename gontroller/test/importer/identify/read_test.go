package identify_test

import (
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const sidecarXMP = `<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?>
<x:xmpmeta xmlns:x='adobe:ns:meta/'><rdf:RDF xmlns:rdf='http://www.w3.org/1999/02/22-rdf-syntax-ns#'>
<rdf:Description rdf:about='' xmlns:exif='http://ns.adobe.com/exif/1.0/'>
<exif:DateTimeOriginal>2024-05-06T07:08:09</exif:DateTimeOriginal>
</rdf:Description></rdf:RDF></x:xmpmeta><?xpacket end='w'?>`

// One exiftool call reads the whole group, as numbers (-n): the item's metadata
// package has the composite GPS signed by its Ref (south and west negative), the
// orientation 1–8, the sidecar's date over the main file's; only the declared tags
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
		"-Orientation#=6", "-DateTimeOriginal=2020:01:01 00:00:00", photo).CombinedOutput(); err != nil {
		t.Fatalf("exiftool write: %v %s", err, out)
	}
	xmp := filepath.Join(dir, "a.xmp")
	if err := os.WriteFile(xmp, []byte(sidecarXMP), 0o644); err != nil {
		t.Fatal(err)
	}

	it := identifyGroup(t, t.TempDir(), photo, xmp)
	if it == nil {
		t.Fatal("no item")
	}
	for tag, want := range map[string]string{
		"GPSLatitude": "-33.8688", "GPSLongitude": "-151.2093", "Orientation": "6",
		"DateTimeOriginal": "2024:05:06 07:08:09", // the sidecar's, over the main file's
	} {
		if got := it.GetExif(tag); got != want {
			t.Errorf("%s %q, want %q", tag, got, want)
		}
	}
	if got := it.GetExif("FileSize"); got != "" {
		t.Errorf("an undeclared tag reached the package: FileSize %q", got)
	}
	if it.Item.MimeType != "image/jpeg" {
		t.Errorf("MIME type %q", it.Item.MimeType)
	}
}
