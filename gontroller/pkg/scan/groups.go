package scan

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"perceptrail/gontroller/pkg/model/dto"
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

var errPhotosNotImplemented = errors.New("apple photos grouper: not implemented yet")

func (photosGrouper) Expand(ev fileEvent) ([]fileGroup, error) {
	if ev.done != nil {
		return []fileGroup{{done: ev.done}}, nil
	}
	return nil, errPhotosNotImplemented
}

func (photosGrouper) Stop() {}

// genericGrouper: sidecars have the main file's name and sit next to it. Files
// are buffered per directory until the walk has left the directory (WalkDir visits
// subdirectories in between its files), then grouped by name.
type genericGrouper struct {
	pending map[string][]dto.ItemEntry
	dirs    []string // pending directories, outermost first (an ancestor chain)
}

func newGenericGrouper() *genericGrouper {
	return &genericGrouper{pending: map[string][]dto.ItemEntry{}}
}

func (g *genericGrouper) Expand(ev fileEvent) ([]fileGroup, error) {
	if ev.done != nil {
		var out []fileGroup
		for len(g.dirs) > 0 {
			out = append(out, g.flushLast()...)
		}
		return append(out, fileGroup{done: ev.done}), nil
	}
	if shouldSkipPath(ev.entry.Path) {
		return nil, nil
	}

	dir := filepath.Dir(ev.entry.Path)
	var out []fileGroup
	// The walk left every pending directory that is not dir or one of its parents
	for len(g.dirs) > 0 {
		last := g.dirs[len(g.dirs)-1]
		if last == dir || isUnder(dir, last) {
			break
		}
		out = append(out, g.flushLast()...)
	}
	if _, ok := g.pending[dir]; !ok {
		g.dirs = append(g.dirs, dir)
	}
	g.pending[dir] = append(g.pending[dir], ev.entry)
	return out, nil
}

func (g *genericGrouper) flushLast() []fileGroup {
	dir := g.dirs[len(g.dirs)-1]
	g.dirs = g.dirs[:len(g.dirs)-1]
	entries := g.pending[dir]
	delete(g.pending, dir)

	var out []fileGroup
	for _, group := range groupByName(entries) {
		out = append(out, fileGroup{entries: group})
	}
	return out
}

func (g *genericGrouper) Stop() {}

// groupByName groups the files of one directory: "a.jpg", "a.xmp", "a.JPG.xmp" and
// "a.MOV" are one group ("a"); "a.edited.jpg" is another. Case-insensitive.
// Groups come out in name order.
func groupByName(entries []dto.ItemEntry) [][]dto.ItemEntry {
	sorted := append([]dto.ItemEntry(nil), entries...)
	// Fewer dots first: "a.jpg" must be known before "a.jpg.xmp" joins it
	sort.SliceStable(sorted, func(i, j int) bool {
		di, dj := strings.Count(sorted[i].Name, "."), strings.Count(sorted[j].Name, ".")
		if di != dj {
			return di < dj
		}
		return sorted[i].Name < sorted[j].Name
	})

	var groups [][]dto.ItemEntry
	byName := map[string]int{} // full name -> group
	byStem := map[string]int{} // name without its last extension -> group
	for _, e := range sorted {
		name := strings.ToLower(e.Name)
		stem := strings.TrimSuffix(name, filepath.Ext(name))

		gi, ok := byName[stem] // "a.jpg.xmp" joins "a.jpg"
		if !ok {
			gi, ok = byStem[stem] // "a.xmp" joins "a.jpg"
		}
		if !ok {
			gi = len(groups)
			groups = append(groups, nil)
			byStem[stem] = gi
		}
		groups[gi] = append(groups[gi], e)
		byName[name] = gi
	}

	sort.SliceStable(groups, func(i, j int) bool { return minName(groups[i]) < minName(groups[j]) })
	return groups
}

func minName(group []dto.ItemEntry) string {
	m := group[0].Name
	for _, e := range group[1:] {
		m = min(m, e.Name)
	}
	return m
}

// Not gallery sources: .THM posters; Apple Photos generated derivatives (until the
// Apple Photos grouper takes over the library).
func shouldSkipPath(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".thm") {
		return true
	}
	if strings.Contains(lower, ".photoslibrary/resources/") {
		return true
	}
	return false
}
