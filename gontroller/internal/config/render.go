package config

import "runtime"

// Render: the `render` section — the expensive stage (renditions of a plain
// folder's items, from the work queue)
type Render struct {
	// Run it. Default: off — the renderer is a stand-in for now (it links, or
	// copies, each original into the cache)
	Enabled bool `yaml:"enabled"`
	// Items rendered at once. Default: half the CPUs, at least one
	Workers int `yaml:"workers"`
}

// resolve fills the unset fields
func (r *Render) resolve() {
	if r.Workers <= 0 {
		r.Workers = max(runtime.NumCPU()/2, 1)
	}
}
