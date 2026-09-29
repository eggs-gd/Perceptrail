package scan

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"

	"gorm.io/gorm"
)

// files gate: keeps the files table (identity, stat, CheckTime) and lets through
// only groups that need work — so unchanged files never reach exiftool. After the
// end-of-walk marker from every grouper it derives deletions.
type filesGate struct {
	logger   *l.Logger
	branches int // markers to wait for
	markers  int
}

// NewFilesGate: branches is the number of groupers that send an end-of-walk marker
func NewFilesGate(branches int, chin <-chan fileGroup, chout chan<- storedGroup, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, newFilesGate(branches, logger))
}

func newFilesGate(branches int, logger *l.Logger) *filesGate {
	return &filesGate{logger: logger, branches: branches}
}

func (g *filesGate) Decorate(in fileGroup) (storedGroup, error) {
	// A grouper's last group comes with its end-of-walk marker: the group first
	out, err := g.pass(in.entries)
	if in.done != nil {
		g.markers++
		if g.markers == g.branches { // every grouper has flushed: all files are stamped
			g.markers = 0
			g.finalizeWalk(*in.done)
		}
	}
	return out, err
}

// pass stores the group and lets it through if it needs work
func (g *filesGate) pass(entries []dto.ItemEntry) (storedGroup, error) {
	if len(entries) == 0 {
		return storedGroup{}, chain.ErrSkippedItem
	}
	files, changed, err := g.store(entries)
	if err != nil {
		return storedGroup{}, err
	}
	if changed || g.needsProcessing(files) {
		return storedGroup{files: files}, nil
	}
	return storedGroup{}, chain.ErrSkippedItem
}

func (g *filesGate) Stop() {}

// store finds or creates the rows of the group, refreshes their stat and stamps
// them as seen. changed: a new file, or size/mtime differ.
func (g *filesGate) store(entries []dto.ItemEntry) ([]*dto.FileDto, bool, error) {
	now := time.Now()
	changed := false
	files := make([]*dto.FileDto, 0, len(entries))

	for _, e := range entries {
		f, err := filesProxy.GetFileByPath(e.Path)
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if f, err = filesProxy.CreateFile(e); err != nil {
				return nil, false, err
			}
			changed = true
		case err != nil:
			return nil, false, err
		case !f.ModTime.Equal(e.ModTime) || f.Size != e.Size:
			// Store the fresh stat, or every walk sees the file as changed again
			// (and the short hash would use the stale size)
			f.Size, f.ModTime = e.Size, e.ModTime
			changed = true
		}
		f.CheckTime = now
		files = append(files, f)
	}

	if _, err := filesProxy.UpdateFiles(files); err != nil {
		return nil, false, err
	}
	return files, changed, nil
}

// needsProcessing: nothing changed on disk, but the group is not done — a file
// was never linked, or the item is missing or not Ready (new, Dirty, interrupted).
// Groups that are known not to be media stay ignored.
func (g *filesGate) needsProcessing(files []*dto.FileDto) bool {
	main := ""
	for _, f := range files {
		switch {
		case f.LinkedTo == "":
			return true
		case !f.IsIgnored() && main == "":
			main = f.LinkedTo
		}
	}
	if main == "" {
		return false // the whole group is ignored
	}
	item, err := itemsProxy.GetItemByGuid(main)
	if err != nil {
		return errors.Is(err, gorm.ErrRecordNotFound)
	}
	return item.State != dto.Ready
}

// finalizeWalk derives deletions: files not stamped by this walk are gone.
func (g *filesGate) finalizeWalk(result walkResult) {
	if !result.complete {
		g.logger.Warn("Walk incomplete: deletions are not checked")
		return
	}
	if result.files == 0 {
		// An empty root (e.g. an unmounted drive's mount point) must not delete the library
		g.logger.Warn("Walk found no files: deletions are not checked", l.String("path", result.root))
		return
	}
	g.logger.Info("Walk complete", l.Int("files", result.files), l.Int("unreadable", len(result.unreadable)))

	stale, err := filesProxy.GetFilesCheckedBefore(result.started)
	if err != nil {
		g.logger.Error("Deletions: can't read files", l.Error(err))
		return
	}
	gone := goneFiles(stale, result.root, result.unreadable)
	deletedItems, dirtyItems := 0, 0

	for _, f := range gone {
		switch {
		case f.IsIgnored() || f.LinkedTo == "":
		case f.LinkedTo == f.GUID: // main file: the item is gone
			item, err := itemsProxy.GetItemByGuid(f.GUID)
			if err != nil {
				continue // never became an item, or already deleted
			}
			if err := itemsProxy.DeleteItem(item); err != nil {
				g.logger.Error("Deletions: can't delete item", l.String("guid", item.Guid), l.Error(err))
				continue
			}
			deletedItems++
		default: // sidecar: its item must be processed again
			item, err := itemsProxy.GetItemByGuid(f.LinkedTo)
			if err != nil {
				continue
			}
			item.State = dto.Dirty
			if _, err := itemsProxy.UpdateItem(item); err != nil {
				g.logger.Error("Deletions: can't mark item dirty", l.String("guid", item.Guid), l.Error(err))
				continue
			}
			dirtyItems++
		}
	}

	if err := filesProxy.DeleteFiles(gone); err != nil {
		g.logger.Error("Deletions: can't delete files", l.Error(err))
		return
	}
	g.logger.Info("Deletions", l.Int("files", len(gone)), l.Int("items", deletedItems), l.Int("dirty", dirtyItems))
}

// goneFiles keeps the stale files that belong to this root and were not hidden by
// an unreadable directory: only those are known to be deleted.
func goneFiles(stale []*dto.FileDto, root string, unreadable []string) []*dto.FileDto {
	var gone []*dto.FileDto
	for _, f := range stale {
		if !isUnder(f.Path, root) {
			continue // another library root (config changed): not ours to judge
		}
		if slices.ContainsFunc(unreadable, func(dir string) bool { return f.Path == dir || isUnder(f.Path, dir) }) {
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
