package discover

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
)

// SweepStore: what the deletions read and write — the files a walk did not stamp,
// what their being gone means (the model's rule)
type SweepStore interface {
	GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error)
	Gone(files []*dto.FileDto) (deleted, dirty int, err error)
}

// sweep: the deletions after a complete walk — the gate runs it once every
// grouper's marker has reached it (all files stamped)
type sweep struct {
	db     SweepStore
	logger *l.Logger
}

// finalizeWalk derives deletions: files not stamped by this walk are gone.
func (g *sweep) finalizeWalk(result Walk, held []string) {
	if !result.Complete {
		g.logger.Warn("Walk incomplete: deletions are not checked")
		return
	}
	if result.Files == 0 {
		// An empty root (e.g. an unmounted drive's mount point) must not delete the library
		g.logger.Warn("Walk found no files: deletions are not checked", l.String("path", result.Root))
		return
	}
	g.logger.Info("Walk complete", l.Int("files", result.Files), l.Int("unreadable", len(result.Unreadable)))

	stale, err := g.db.GetFilesCheckedBefore(result.Started)
	if err != nil {
		g.logger.Error("Deletions: can't read files", l.Error(err))
		return
	}
	gone := goneFiles(stale, result.Root, result.Unreadable, held)
	deletedItems, dirtyItems, err := g.db.Gone(gone)
	if err != nil {
		g.logger.Error("Deletions failed", l.Error(err))
		return
	}
	g.logger.Info("Deletions", l.Int("files", len(gone)), l.Int("items", deletedItems), l.Int("dirty", dirtyItems))
}

// goneFiles keeps the stale files that belong to this root and were not hidden by
// an unreadable directory: only those are known to be deleted.
func goneFiles(stale []*dto.FileDto, root string, unreadable, held []string) []*dto.FileDto {
	isHeld := make(map[string]bool, len(held))
	for _, p := range held {
		isHeld[p] = true
	}
	var gone []*dto.FileDto
	for _, f := range stale {
		if !isUnder(f.Path, root) {
			continue // another library root (config changed): not ours to judge
		}
		if isHeld[f.Path] || slices.ContainsFunc(unreadable, func(dir string) bool { return f.Path == dir || isUnder(f.Path, dir) }) {
			continue
		}
		gone = append(gone, f)
	}
	return gone
}

// isUnder reports whether path is inside dir
func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
