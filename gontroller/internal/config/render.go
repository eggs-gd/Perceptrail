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
	// Threads of each tool (libvips, ffmpeg). Default: the CPUs over the workers.
	// "The CPUs" are what this process may use (a container's CPU limit) — a
	// container that limits nothing (an LXC) sees the whole host: set both here.
	Threads int `yaml:"threads"`
	// The long side of each rendition, px (never larger than the original), smallest
	// first, each once. Default: [400, 1600] — a tile and the viewer
	Sizes []int `yaml:"sizes"`
	// One format for every rendition: webp (default), jpg
	Format string `yaml:"format"`
	// The vipsthumbnail executable (libvips). Default: "vipsthumbnail" from PATH
	Vipsthumbnail string `yaml:"vipsthumbnail"`
	// The ffmpeg executable, for videos (ffprobe beside it). Default: "ffmpeg" from
	// PATH
	FFmpeg string `yaml:"ffmpeg"`
	// The long side of a video's rendition, px (never larger than the original).
	// Default: 1280 (720p)
	Video int `yaml:"video"`
}

const (
	defaultVipsthumbnail = "vipsthumbnail"
	defaultFFmpeg        = "ffmpeg"
	defaultVideo         = 1280
)

var (
	defaultSizes  = []int{400, 1600}
	renderFormats = []string{"webp", "jpg"}
)

// Enabled: render runs
func (r Render) Enabled() bool { return r.Enable == nil || *r.Enable }

// FFprobe: the ffprobe beside the ffmpeg (from PATH when ffmpeg is)
func (r Render) FFprobe() string {
	if filepath.Base(r.FFmpeg) == r.FFmpeg {
		return "ffprobe"
	}
	return filepath.Join(filepath.Dir(r.FFmpeg), "ffprobe")
}

// resolve fills the unset fields and checks the rest (abs: a path against the
// config's directory)
func (r *Render) resolve(abs func(string) string) error {
	cpus := runtime.GOMAXPROCS(0) // the CPUs this process may use (a cgroup's quota too)
	if r.Workers <= 0 {
		r.Workers = max(cpus/2, 1)
	}
	if r.Threads <= 0 {
		r.Threads = max(cpus/r.Workers, 1)
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
	if r.Video <= 0 {
		r.Video = defaultVideo
	}
	r.Vipsthumbnail = tool(r.Vipsthumbnail, defaultVipsthumbnail, abs)
	r.FFmpeg = tool(r.FFmpeg, defaultFFmpeg, abs)
	return nil
}

// tool: an executable as configured — its default when unset; a bare command name is
// looked up in PATH, only a path is resolved
func tool(path, fallback string, abs func(string) string) string {
	if path == "" {
		return fallback
	}
	if filepath.Base(path) != path {
		return abs(path)
	}
	return path
}
