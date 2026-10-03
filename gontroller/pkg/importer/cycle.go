package importer

import (
	"time"

	"perceptrail/gontroller/pkg/importer/exif"
	"perceptrail/gontroller/pkg/importer/walk"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
)

// The walk cycle: the walker walks when asked; once the walk's flush reached the end
// of the chain (every group of it went through every step), its deletions, the
// perceptors' rows of gone items, the pause, the next walk.

// CycleStore: what the cycle asks of the model — the files a walk did not stamp, what
// their being gone means, the items (the perceptors' bookkeeping)
type CycleStore interface {
	exif.Items
	GetFilesCheckedBefore(t time.Time) ([]*dto.FileDto, error)
	Gone(files []*dto.FileDto) (deleted, dirty int, err error)
}

type cycle struct {
	db     CycleStore
	walker *walk.Walker
	rescan time.Duration
	logger *l.Logger
}

// walked: the walk's flush reached the end of the chain
func (c *cycle) walked() {
	deleteGone(c.db, c.walker.Last(), c.logger)
	exif.Prune(c.db, c.logger)
	time.AfterFunc(c.rescan, c.walker.Next)
}

// deleteGone: the files the walk did not see that it says are gone (walk.Gone: a
// complete walk, under its root, not under an unreadable directory) go by the
// model's rules (Gone: a main file's item deleted, a sidecar's item processed again)
func deleteGone(db CycleStore, r walk.Result, logger *l.Logger) {
	switch {
	case !r.Complete:
		logger.Warn("Walk incomplete: deletions are not checked")
		return
	case r.Files == 0:
		logger.Warn("Walk found no files: deletions are not checked", l.String("path", r.Root))
		return
	}
	logger.Info("Walk complete", l.Int("files", r.Files), l.Int("unreadable", len(r.Unreadable)))
	stale, err := db.GetFilesCheckedBefore(r.Started)
	if err != nil {
		logger.Error("Deletions: can't read files", l.Error(err))
		return
	}
	gone := walk.Gone(r, stale)
	deleted, dirty, err := db.Gone(gone)
	if err != nil {
		logger.Error("Deletions failed", l.Error(err))
		return
	}
	logger.Info("Deletions", l.Int("files", len(gone)), l.Int("items", deleted), l.Int("dirty", dirty))
}
