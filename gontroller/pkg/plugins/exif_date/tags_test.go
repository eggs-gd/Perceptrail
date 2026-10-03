package exif_date

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
		{"DateTimeOriginal": "2024:01:02 10:00:00", "GPSLatitude": "50.45", "GPSLongitude": "30.516667"},
	} {
		r := exif_coretest.NewRecorder(exif)
		if _, err := (&datesExtractor{}).Decorate(r); err != nil {
			t.Fatal(err)
		}
		r.AssertDeclared(t, Perceptor.(api.ExifTagger))
	}
}
