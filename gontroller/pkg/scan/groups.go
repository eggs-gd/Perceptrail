package scan

import (
	"errors"
	"path/filepath"
	"strings"

	"perceptrail/gontroller/pkg/model/dto"

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

// photosGrouper will group an Apple Photos library by its database (assets:
// original, render, Live Photo video). Not implemented: photosLibraryEnabled
// keeps files away from it; it only passes the marker on.
type photosGrouper struct{}

func NewPhotosGrouper(chin <-chan fileEvent, chout chan<- FileGroup) chain.Processor {
	return chain.NewDecorator(chin, chout, photosGrouper{})
}

var errPhotosNotImplemented = errors.New("apple photos grouper: not implemented yet")

func (photosGrouper) Decorate(ev fileEvent) (FileGroup, error) {
	if ev.done != nil {
		return FileGroup{Done: ev.done}, nil
	}
	return FileGroup{}, errPhotosNotImplemented
}

func (photosGrouper) Stop() {}

// genericGrouper: sidecars have the main file's name and sit next to it, and the
// walk lists a directory in name order — so a group's files come one after
// another. One group is open; a file that does not belong to it closes it (the
// group goes out) and opens the next. The marker goes out with the last group.
type genericGrouper struct {
	open []*dto.FileDto
}

func NewGenericGrouper(chin <-chan fileEvent, chout chan<- FileGroup) chain.Processor {
	return chain.NewDecorator(chin, chout, newGenericGrouper())
}

func newGenericGrouper() *genericGrouper {
	return &genericGrouper{}
}

func (g *genericGrouper) Decorate(ev fileEvent) (FileGroup, error) {
	if ev.done != nil {
		last := g.open
		g.open = nil
		return FileGroup{Files: last, Done: ev.done}, nil
	}
	if shouldSkipPath(ev.entry.Path) {
		return FileGroup{}, chain.ErrSkippedItem
	}
	file := &dto.FileDto{ItemEntry: ev.entry}
	if len(g.open) == 0 || sameGroup(g.open, file) {
		g.open = append(g.open, file)
		return FileGroup{}, chain.ErrSkippedItem // not complete yet
	}
	closed := g.open
	g.open = []*dto.FileDto{file}
	return FileGroup{Files: closed}, nil
}

func (g *genericGrouper) Stop() {}

// sameGroup: the file sits in the group's directory and its name without the last
// extension is the name or the stem of a group member: "a.jpg", "a.xmp",
// "a.jpg.xmp", "a.MOV" are one group, "a.edited.jpg" is not. Case-insensitive.
func sameGroup(group []*dto.FileDto, e *dto.FileDto) bool {
	if filepath.Dir(group[0].Path) != filepath.Dir(e.Path) {
		return false
	}
	stem := nameStem(e.Name)
	for _, m := range group {
		if stem == strings.ToLower(m.Name) || stem == nameStem(m.Name) {
			return true
		}
	}
	return false
}

// nameStem: the lower-case name without its last extension
func nameStem(name string) string {
	name = strings.ToLower(name)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// Not gallery sources: .THM posters; everything in an Apple Photos library except
// originals/ (derivatives, renders, caches, internal/ and scopes/ images of Apple's
// own) — until the Apple Photos grouper takes over the library and links the
// derivatives to their asset from the library's DB.
func shouldSkipPath(path string) bool {
	if strings.HasSuffix(strings.ToLower(path), ".thm") {
		return true
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if strings.HasSuffix(strings.ToLower(part), ".photoslibrary") {
			return i+1 >= len(parts) || parts[i+1] != "originals"
		}
	}
	return false
}
