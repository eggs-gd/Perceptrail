package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	l "github.com/eggs-gd/go-zap-decor"
)

const (
	sweepEvery = time.Hour // the cache swept at start, then this often
	grace      = time.Hour // what changed this recently stays: a worker may be writing it
)

// sweep: the maintenance of render's part of the cache. The model drops the rows no
// one shows any more (Prune: another version replaced them, their item went); then
// every file under r/ the database no longer lists goes, and the directories left
// empty — old versions, deleted items, anything a crash left behind.
//
// Not obvious:
//   - A file or directory changed within the grace stays — a worker writes its
//     renditions (and makes their directory) before Finish lists them.
//   - Removing a file touches its directory: a directory the sweep emptied goes at
//     once, whatever its time says.
func (s *Service) sweep() {
	rows, err := s.db.Prune()
	if err != nil {
		s.logger.Error("Render: stale rows not pruned", l.Error(err))
		return
	}
	paths, err := s.db.RenditionPaths()
	if err != nil {
		s.logger.Error("Render: the renditions not read for the sweep", l.Error(err))
		return
	}
	keep := make(map[string]bool, len(paths))
	for _, p := range paths {
		keep[p] = true
	}
	dirs, emptied, files := sweepFiles(s.cfg.CacheDir(), keep)
	sweepDirs(dirs, emptied)
	if rows > 0 || files > 0 {
		s.logger.Info("Render: the cache swept", l.Int("rows", int(rows)), l.Int("files", files))
	}
}

// recent: changed within the grace
func recent(d fs.DirEntry) bool {
	info, err := d.Info()
	return err != nil || time.Since(info.ModTime()) < grace
}

// sweepFiles: every file under the cache's r/ that keep does not list removed (a
// recent one stays); the directories seen, the ones it took something out of
func sweepFiles(cache string, keep map[string]bool) (dirs []string, emptied map[string]bool, files int) {
	root := filepath.Join(cache, "r")
	emptied = map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil: // gone meanwhile, or no cache yet: nothing to sweep there
		case d.IsDir():
			if path != root {
				dirs = append(dirs, path)
			}
		case stale(cache, path, d, keep) && os.Remove(path) == nil:
			files++
			emptied[filepath.Dir(path)] = true
		}
		return nil //nolint:nilerr // an entry not read is skipped: the sweep goes on
	})
	return dirs, emptied, files
}

// stale: a file of the cache that keep does not list, older than the grace
func stale(cache, path string, d fs.DirEntry, keep map[string]bool) bool {
	rel, err := filepath.Rel(cache, path)
	return err == nil && !keep[rel] && !recent(d)
}

// sweepDirs: the directories left empty go, the deepest first — one the sweep
// emptied at once, another after the grace
func sweepDirs(dirs []string, emptied map[string]bool) {
	for _, dir := range slices.Backward(dirs) {
		info, err := os.Stat(dir)
		if err != nil || !emptied[dir] && time.Since(info.ModTime()) < grace {
			continue
		}
		if os.Remove(dir) == nil { // only an empty one goes
			emptied[filepath.Dir(dir)] = true
		}
	}
}
