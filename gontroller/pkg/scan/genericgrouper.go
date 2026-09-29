package scan

import (
	"path/filepath"
	"strings"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
)

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
