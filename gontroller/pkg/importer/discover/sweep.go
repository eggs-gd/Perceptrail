package discover

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"

	l "github.com/eggs-gd/perceplib/logger"
)

// SweepStore: what the deletions read and write — the files a walk did not stamp,
// the items they belonged to, and every item's GUID (the perceptors' rows to keep)
type SweepStore interface {
	GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error)
	DeleteFiles(files []*dto.FileDto) error
	CountLinkedFiles(guid string) (int64, error)
	GetItemByGuid(guid string) (*dto.ItemDto, error)
	UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error)
	DeleteItem(item *dto.ItemDto) error
	GetAllGuids() ([]string, error)
}

// Pruner: the perceptors' rows of items not kept are dropped
type Pruner interface {
	Prune(keep func(guid string) bool)
}

// sweep: the deletions after a complete walk — the gate runs it once every
// grouper's marker has reached it (all files stamped)
type sweep struct {
	db         SweepStore
	perceptors Pruner
	logger     *l.Logger
}

// finalizeWalk derives deletions: files not stamped by this walk are gone.
func (g *sweep) finalizeWalk(result providers.Walk, held []string) {
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
	deletedItems, dirtyItems := 0, 0

	for _, f := range gone {
		switch {
		case f.IsIgnored() || f.LinkedTo == "":
		case f.LinkedTo == f.GUID: // main file: the item is gone
			item, err := g.db.GetItemByGuid(f.GUID)
			if err != nil {
				continue // never became an item, or already deleted
			}
			if err := g.db.DeleteItem(item); err != nil {
				g.logger.Error("Deletions: can't delete item", l.String("guid", item.Guid), l.Error(err))
				continue
			}
			deletedItems++
		default: // sidecar: its item must be processed again
			item, err := g.db.GetItemByGuid(f.LinkedTo)
			if err != nil {
				continue
			}
			item.State = dto.Dirty
			if _, err := g.db.UpdateItem(item); err != nil {
				g.logger.Error("Deletions: can't mark item dirty", l.String("guid", item.Guid), l.Error(err))
				continue
			}
			dirtyItems++
		}
	}

	if err := g.db.DeleteFiles(gone); err != nil {
		g.logger.Error("Deletions: can't delete files", l.Error(err))
		return
	}
	// An item with no files left is gone too (a keyed asset: every file is "linked",
	// none is "main" by its own GUID)
	for _, f := range gone {
		if f.LinkedTo == "" || f.IsIgnored() {
			continue
		}
		if n, err := g.db.CountLinkedFiles(f.LinkedTo); err == nil && n == 0 {
			if item, err := g.db.GetItemByGuid(f.LinkedTo); err == nil {
				if err := g.db.DeleteItem(item); err == nil {
					deletedItems++
				}
			}
		}
	}
	g.logger.Info("Deletions", l.Int("files", len(gone)), l.Int("items", deletedItems), l.Int("dirty", dirtyItems))
	g.pruneStores()
}

// pruneStores: the perceptors' rows of items that are gone
func (g *sweep) pruneStores() {
	guids, err := g.db.GetAllGuids()
	if err != nil {
		g.logger.Error("Perceptor storage: can't read items", l.Error(err))
		return
	}
	keep := make(map[string]bool, len(guids))
	for _, guid := range guids {
		keep[guid] = true
	}
	g.perceptors.Prune(func(guid string) bool { return keep[guid] })
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
