package size

import (
	"testing"

	"perceptrail/gontroller/pkg/perceptor/builtintest"

	"github.com/eggs-gd/perceplib/api"
)

// It reads only the tags it declares: the core reads nothing else
func TestReadsDeclaredTags(t *testing.T) {
	for _, exif := range []map[string]string{
		{},
		{"ImageWidth": "4000", "ImageHeight": "3000"},
	} {
		r := builtintest.NewRecorder(exif)
		if _, err := (&sizesExtractor{}).Decorate(r); err != nil {
			t.Fatal(err)
		}
		r.AssertDeclared(t, Perceptor.(api.ExifTagger))
	}
}
