package identify

import (
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// A tag comes from the first that has it: the source, a metadata sidecar, the main
// file, then a derivative
func TestMergePriority(t *testing.T) {
	file := func(role string) *dto.FileDto { return &dto.FileDto{Role: role} }
	d := &draft{
		Asset: dto.Asset{
			Meta:  api.RawExif{"DateTimeOriginal": []byte("source")},
			Files: []*dto.FileDto{file(dto.RoleOriginal), file(dto.RoleStill), file(dto.RoleMeta)},
		},
		Exif: []api.RawExif{
			{"DateTimeOriginal": []byte("main"), "Orientation": []byte("main"), "Make": []byte("main"), "Error": []byte("main")},
			{"Orientation": []byte("derivative"), "Make": []byte("derivative"), "Lens": []byte("derivative")},
			{"DateTimeOriginal": []byte("xmp"), "Orientation": []byte("xmp")},
		},
	}
	merge(d, map[string]bool{"DateTimeOriginal": true, "Orientation": true, "Make": true, "Lens": true})
	it := yield(d)
	if got := it.GetExif("Error"); got != "" {
		t.Errorf("an undeclared tag reaches the package: %q", got)
	}
	for tag, want := range map[string]string{
		"DateTimeOriginal": "source",
		"Orientation":      "xmp",
		"Make":             "main",
		"Lens":             "derivative",
	} {
		if got := it.GetExif(tag); got != want {
			t.Errorf("%s: %q, want %q", tag, got, want)
		}
	}
}
