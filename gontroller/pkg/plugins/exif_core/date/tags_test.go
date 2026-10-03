package date

import (
	"testing"

	"perceptrail/gontroller/pkg/plugins/exif_core/exif_coretest"

	"github.com/eggs-gd/perceplib/api"
)

// It reads only the tags it declares: the core reads nothing else
func TestReadsDeclaredTags(t *testing.T) {
	for _, exif := range []map[string]string{
		{},
		{"MIMEType": "video/quicktime"},
		{"MIMEType": "video/quicktime", "CreateDate": "2024:01:02 10:00:00"},
		{"DateTimeOriginal": "2024:01:02 10:00:00"},
		{"DateTimeOriginal": "2024:01:02 10:00:00", "GPSLatitude": "50 deg 27' 0.00\" N", "GPSLongitude": "30 deg 31' 0.00\" E"},
	} {
		r := exif_coretest.NewRecorder(exif)
		if _, err := (&datesExtractor{}).Decorate(r); err != nil {
			t.Fatal(err)
		}
		r.AssertDeclared(t, Perceptor.(api.ExifTagger))
	}
}
