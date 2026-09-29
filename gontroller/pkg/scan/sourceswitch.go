package scan

import (
	"path/filepath"
	"strings"

	"github.com/eggs-gd/perceplib/chain"
)

// groups: a switch by source, one grouper per source. Every grouper turns files
// into groups (a main file with its sidecars, not ranked yet); the gate after
// them is shared.

// Grouper branches of the source switch
const (
	branchGeneric = iota
	branchPhotos
	groupBranches // how many; the gate waits for the end-of-walk marker from each
)

// photosLibraryEnabled routes files inside *.photoslibrary to the Apple Photos
// grouper. Off until that grouper exists: the library goes to generic, as before.
const photosLibraryEnabled = false

type sourceSwitch struct{}

// NewSourceSwitch: every file to the grouper of its source
func NewSourceSwitch(chin <-chan fileEvent, toGeneric, toPhotos chan<- fileEvent) chain.Processor {
	return chain.NewSwitch(chin, []chan<- fileEvent{branchGeneric: toGeneric, branchPhotos: toPhotos}, sourceSwitch{})
}

// Switch sends a file to the grouper of its source; the end-of-walk marker goes
// to every grouper, so each can flush what it holds.
func (sourceSwitch) Switch(ev fileEvent) (map[int]fileEvent, error) {
	if ev.done != nil {
		all := make(map[int]fileEvent, groupBranches)
		for b := range groupBranches {
			all[b] = ev
		}
		return all, nil
	}
	if photosLibraryEnabled && inPhotosLibrary(ev.entry.Path) {
		return map[int]fileEvent{branchPhotos: ev}, nil
	}
	return map[int]fileEvent{branchGeneric: ev}, nil
}

func (sourceSwitch) Stop() {}

// inPhotosLibrary: the path is inside an Apple Photos library bundle
func inPhotosLibrary(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasSuffix(strings.ToLower(part), ".photoslibrary") {
			return true
		}
	}
	return false
}
