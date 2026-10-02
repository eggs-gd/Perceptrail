// Package photokit asks Apple Photos (PhotoKit, macOS only) for an asset's
// renditions it keeps only in iCloud: Photos downloads them into its own library,
// the next walk finds the files. We never write the library ourselves, and keep no
// copy of what Photos keeps. Elsewhere (Linux, Docker) every request fails with
// ErrUnavailable and only what is on disk is served.
//
// Findings "PhotoKit spike": what each request makes local and how long it takes.
package photokit

import "errors"

// ErrUnavailable: no PhotoKit here (not macOS), or no access to Photos
var ErrUnavailable = errors.New("photokit: not available")

// Video delivery modes. Automatic and high quality download the original: high
// only when the original is asked for.
const (
	VideoOriginal = 1 // the original file
	VideoMedium   = 2 // 720p — HEVC for iPhone videos, H.264 for the rest
	VideoFast     = 3 // H.264 360p
)

// Library asks Photos; the zero value is ready
type Library struct{}

// Authorize asks for read access to Photos once (see Authorize)
func (Library) Authorize() bool { return Authorize() }

// Image makes the asset's image of at most size×size local (Photos' ~2048 px
// rendition when only in iCloud) and returns it as JPEG: drawn from a local
// original (a HEIC) Photos writes no file — the JPEG is all there is to show
func (Library) Image(uuid string, size int) ([]byte, error) { return image(uuid, size) }

// Full is the biggest of what the user sees — the current version (the edit,
// cropped) at full resolution — as JPEG: any browser shows it. Photos downloads the
// original into its library to draw it when it has nothing local that big.
func (Library) Full(uuid string) ([]byte, error) { return image(uuid, 0) }

// Video makes the asset's video in that mode local and returns its file
func (Library) Video(uuid string, mode int) (string, error) { return video(uuid, mode) }

// Live makes a Live Photo's motion local
func (Library) Live(uuid string) error { return live(uuid) }
