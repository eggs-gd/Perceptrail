package identify

import (
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// Merge: the merge step's logic — the asset's metadata package, one map. A tag is
// taken from the first that has it: the source's own metadata (the Photos DB: what
// the user corrected there), the metadata sidecars (.xmp: they override the main
// file without changing it), the main file, then the derivatives (a fallback only:
// a JPEG's size or orientation must not override its RAW's). After classify: the
// main file and the roles are known. Only the declared tags (the perceptors') make
// the package: identify's own tags stay with the files.
type Merge struct {
	tags map[string]bool
}

func NewMerge(tags []string) *Merge {
	m := &Merge{tags: make(map[string]bool, len(tags))}
	for _, t := range tags {
		m.tags[t] = true
	}
	return m
}

func (m *Merge) Decorate(d *draft) (*draft, error) {
	// Highest first: the source, the sidecars, the main file, the derivatives
	layers := []api.RawExif{d.Meta}
	for i := 1; i < len(d.Files); i++ {
		if d.Files[i].Role == dto.RoleMeta {
			layers = append(layers, d.Exif[i])
		}
	}
	layers = append(layers, d.Exif[0])
	for i := 1; i < len(d.Files); i++ {
		if d.Files[i].Role != dto.RoleMeta {
			layers = append(layers, d.Exif[i])
		}
	}

	d.Merged = api.RawExif{}
	for _, layer := range layers {
		for k, v := range layer {
			if _, taken := d.Merged[k]; !taken && m.tags[k] {
				d.Merged[k] = v
			}
		}
	}
	return d, nil
}

func (m *Merge) Stop() {}
