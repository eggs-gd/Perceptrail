package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
)

// Render: the `render` section — the expensive stage (renditions of a plain
// folder's items, from the work queue)
type Render struct {
	// Run it. Default: on (a library's items are rendered by their library)
	Enable *bool `yaml:"enabled"`
	// Items rendered at once. Default: half the CPUs, at least one
	Workers int `yaml:"workers"`
	// The long side of each rendition, px (never larger than the original), smallest
	// first, each once. Default: [400, 1600] — a tile and the viewer
	Sizes []int `yaml:"sizes"`
	// One format for every rendition: webp (default), jpg
	Format string `yaml:"format"`
	// The vipsthumbnail executable (libvips). Default: "vipsthumbnail" from PATH
	Vipsthumbnail string `yaml:"vipsthumbnail"`
}

const defaultVipsthumbnail = "vipsthumbnail"

var (
	defaultSizes  = []int{400, 1600}
	renderFormats = []string{"webp", "jpg"}
)

// Enabled: render runs
func (r Render) Enabled() bool { return r.Enable == nil || *r.Enable }

// resolve fills the unset fields and checks the rest (abs: a path against the
// config's directory)
func (r *Render) resolve(abs func(string) string) error {
	if r.Workers <= 0 {
		r.Workers = max(runtime.NumCPU()/2, 1)
	}
	if len(r.Sizes) == 0 {
		r.Sizes = defaultSizes
	}
	for _, size := range r.Sizes {
		if size <= 0 {
			return fmt.Errorf("render size %d: must be positive", size)
		}
	}
	r.Sizes = slices.Compact(slices.Sorted(slices.Values(r.Sizes))) // two of one size would clash
	if r.Format == "" {
		r.Format = renderFormats[0]
	}
	if !slices.Contains(renderFormats, r.Format) {
		return fmt.Errorf("render format %q: one of %v", r.Format, renderFormats)
	}
	if r.Vipsthumbnail == "" {
		r.Vipsthumbnail = defaultVipsthumbnail
	}
	// A bare command name is looked up in PATH; only paths are resolved
	if filepath.Base(r.Vipsthumbnail) != r.Vipsthumbnail {
		r.Vipsthumbnail = abs(r.Vipsthumbnail)
	}
	return nil
}
