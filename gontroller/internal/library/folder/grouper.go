// Package folder: the plain folder — the provider that takes every file no other
// one claimed (the last in the switch). Its grouper: sidecars have the main file's
// name and sit next to it, and the walk lists a directory in name order — so a
// group's files come one after another. One group is open; a file that does not
// belong to it closes it (the group goes out) and opens the next; the walk's end
// (Flush) sends the last one. A file the walk found missing passes through as it is
// (an asset of its own, Missing).
package folder

import (
	"path/filepath"
	"strings"

	"perceptrail/gontroller/internal/model/dto"

	chain "github.com/eggs-gd/go-chain"
)

type Grouper struct {
	open []*dto.FileDto
}

func (g *Grouper) Decorate(file dto.WalkedFile) (dto.Asset, error) {
	switch {
	case file.Missing:
		return dto.Asset{Missing: []*dto.FileDto{file.FileDto}}, nil
	case shouldSkipPath(file.Path):
		return dto.Asset{}, chain.ErrSkippedItem
	case len(g.open) == 0 || sameGroup(g.open, file.FileDto):
		g.open = append(g.open, file.FileDto)
		return dto.Asset{}, chain.ErrSkippedItem // not complete yet
	}
	closed := g.open
	g.open = []*dto.FileDto{file.FileDto}
	return dto.Asset{Files: closed}, nil
}

// Flush: the walk ended — the last open group goes out
func (g *Grouper) Flush() ([]dto.Asset, error) {
	last := g.open
	g.open = nil
	if len(last) == 0 {
		return nil, nil
	}
	return []dto.Asset{{Files: last}}, nil
}

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

// Not gallery sources: .THM posters (next to a video they are its derivative; in an
// Apple Photos library the apple grouper handles them)
func shouldSkipPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".thm")
}
