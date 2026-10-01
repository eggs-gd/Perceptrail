package groups

import (
	"path/filepath"
	"strings"

	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"
)

// Package groups turns found files into whole assets (flow.FileGroup): a switch
// by source, one grouper per source in a sub-package (generic, apple, …). Every
// grouper keeps a buffer of open groups and sends a group when it is complete;
// the files gate after them is shared.

// Grouper branches of the source switch
const (
	BranchGeneric = iota
	BranchApple
	Branches // how many; the files gate waits for the end-of-walk marker from each
)

// appleEnabled routes files inside *.photoslibrary to the Apple Photos grouper
const appleEnabled = true

type SourceSwitch struct{}

// NewSourceSwitch: every file to the grouper of its source
func NewSourceSwitch(chin <-chan flow.FileEvent, toGeneric, toApple chan<- flow.FileEvent) chain.Processor {
	return chain.NewSwitch(chin, []chan<- flow.FileEvent{BranchGeneric: toGeneric, BranchApple: toApple}, SourceSwitch{})
}

// Switch sends a file to the grouper of its source; the end-of-walk marker goes
// to every grouper, so each can flush what it holds.
func (SourceSwitch) Switch(ev flow.FileEvent) (map[int]flow.FileEvent, error) {
	if ev.Done != nil {
		all := make(map[int]flow.FileEvent, Branches)
		for b := range Branches {
			all[b] = ev
		}
		return all, nil
	}
	if appleEnabled && inPhotosLibrary(ev.Entry.Path) {
		return map[int]flow.FileEvent{BranchApple: ev}, nil
	}
	return map[int]flow.FileEvent{BranchGeneric: ev}, nil
}

func (SourceSwitch) Stop() {}

// inPhotosLibrary: the path is inside an Apple Photos library bundle
func inPhotosLibrary(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasSuffix(strings.ToLower(part), ".photoslibrary") {
			return true
		}
	}
	return false
}
