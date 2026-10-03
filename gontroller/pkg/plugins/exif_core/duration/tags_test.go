package duration

import (
	"testing"

	"perceptrail/gontroller/pkg/plugins/exif_core/exif_coretest"

	"github.com/eggs-gd/perceplib/api"
)

// It reads only the tags it declares: the core reads nothing else
func TestReadsDeclaredTags(t *testing.T) {
	for _, exif := range []map[string]string{
		{},
	} {
		r := exif_coretest.NewRecorder(exif)
		if _, err := (&durationExtractor{}).Decorate(r); err != nil {
			t.Fatal(err)
		}
		r.AssertDeclared(t, Perceptor.(api.ExifTagger))
	}
}
