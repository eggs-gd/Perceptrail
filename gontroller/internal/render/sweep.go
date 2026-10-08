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
	root := filepath.Join(s.cfg.CacheDir(), "r")
	var dirs []string
	emptied := map[string]bool{} // directories the sweep took something out of
	files := 0
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // gone meanwhile, or no cache yet
		}
		if d.IsDir() {
			if path != root {
				dirs = append(dirs, path)
			}
			return nil
		}
		rel, err := filepath.Rel(s.cfg.CacheDir(), path)
		if err != nil || keep[rel] || recent(d) {
			return nil
		}
		if os.Remove(path) == nil {
			files++
			emptied[filepath.Dir(path)] = true
		}
		return nil
	})
	for _, dir := range slices.Backward(dirs) { // the deepest first
		info, err := os.Stat(dir)
		if err != nil || !emptied[dir] && time.Since(info.ModTime()) < grace {
			continue
		}
		if os.Remove(dir) == nil { // only an empty one goes
			emptied[filepath.Dir(dir)] = true
		}
	}
	if rows > 0 || files > 0 {
		s.logger.Info("Render: the cache swept", l.Int("rows", int(rows)), l.Int("files", files))
	}
}

// recent: changed within the grace
func recent(d fs.DirEntry) bool {
	info, err := d.Info()
	return err != nil || time.Since(info.ModTime()) < grace
}
