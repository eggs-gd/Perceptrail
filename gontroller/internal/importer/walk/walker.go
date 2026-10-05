// Package walk: the chain's entry — the library's files, one walk per pass. It reads
// the files table once; every file it sees is sent on as its row — a new file's
// row created, a changed one's stat saved (Changed), page by page, one transaction
// each; an unchanged file costs no write. After a complete walk, the rows it did
// not see that it says are gone are sent too (Missing: their provider decides, the
// gate deletes them); then it returns and its output closes.
package walk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"perceptrail/gontroller/internal/model/dto"

	chain "github.com/eggs-gd/go-chain"

	l "github.com/eggs-gd/go-zap-decor"
)

// Result describes a finished walk. Deletions may be derived from it only if the
// walk was complete: a cancelled walk or an unreadable root says nothing about which
// files are gone.
type Result struct {
	Root string
	// The walk reached the end
	Complete bool
	// Files seen (before grouping and filtering)
	Files int
	// Directories that could not be read: their files are not "deleted"
	Unreadable []string
}

// Store: the files table as a walk reads and writes it
type Store interface {
	GetAllFiles() ([]*dto.FileDto, error)
	GetFilesByID(ids []uint) ([]*dto.FileDto, error)
	CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error)
	SaveStats(files []*dto.FileDto) error
}

// page: how many files a walk sends at once, their rows written in one transaction
// first
const page = 256

// Walker: the walk step's logic (a chain.EntryPoint)
type Walker struct {
	logger *l.Logger
	root   string
	skip   map[string]bool // the directories not entered
	db     Store
}

// New: the chain's entry — out gets every file of a walk, then the gone ones
// skipped: directories not entered (the libraries' own, without their media); the
// rows under them are missing
func New(db Store, root string, skipped []string, logger *l.Logger, out chan<- dto.WalkedFile) chain.Processor {
	skip := make(map[string]bool, len(skipped))
	for _, dir := range skipped {
		skip[filepath.Clean(dir)] = true
	}
	return chain.NewEntryPoint(out, &Walker{logger: logger, root: root, skip: skip, db: db})
}

// Start: one walk — the files seen, then the missing ones
func (m *Walker) Start(ctx context.Context, emit func(dto.WalkedFile) bool) error {
	rows, err := m.db.GetAllFiles()
	if err != nil {
		return err // nothing walked: no deletions either
	}
	// The rows not seen yet: what is left after the walk is missing
	unseen := make(map[string]*dto.FileDto, len(rows))
	for _, f := range rows {
		unseen[f.Path] = f
	}
	var entries []dto.ItemEntry
	send := func() bool {
		files := m.seen(entries, unseen)
		entries = entries[:0]
		for _, f := range files {
			if !emit(dto.WalkedFile{FileDto: f}) {
				return false
			}
		}
		return true
	}
	r := m.walk(ctx, func(e dto.ItemEntry) bool {
		entries = append(entries, e)
		return len(entries) < page || send()
	})
	if !send() {
		return nil
	}
	missing := m.missing(r, unseen)
	for _, f := range missing {
		if !emit(dto.WalkedFile{FileDto: f, Missing: true}) {
			break
		}
	}
	if r.Complete {
		m.logger.Info("Walk complete", l.Int("files", r.Files), l.Int("unreadable", len(r.Unreadable)), l.Int("gone", len(missing)))
	}
	return nil
}

// seen: a page of the walk as rows, in its order — a new file's row created, a
// changed one's stat saved (Changed: its size or mtime differ); unchanged rows are
// not written. A page whose rows could not be written is not sent: the next pass
// sees it again.
func (m *Walker) seen(entries []dto.ItemEntry, unseen map[string]*dto.FileDto) []*dto.FileDto {
	files := make([]*dto.FileDto, len(entries))
	var fresh []dto.ItemEntry
	var changed []*dto.FileDto
	for i, e := range entries {
		f, known := unseen[e.Path]
		if !known {
			fresh = append(fresh, e)
			continue
		}
		delete(unseen, e.Path)
		if !f.ModTime.Equal(e.ModTime) || f.Size != e.Size {
			// The fresh stat stored, or every walk sees the file as changed again (and
			// the fingerprint would use the stale size)
			f.Size, f.ModTime, f.Changed = e.Size, e.ModTime, true
			changed = append(changed, f)
		}
		files[i] = f
	}
	created, err := m.db.CreateFiles(fresh)
	if err == nil {
		err = m.db.SaveStats(changed)
	}
	if err != nil {
		m.logger.Error("Files not stored", l.String("from", entries[0].Path), l.Int("files", len(entries)), l.Error(err))
		return nil
	}
	for i := range files { // the new rows in the walk's order
		if files[i] == nil {
			files[i], created = created[0], created[1:]
		}
	}
	return files
}

// missing: the rows the walk did not see that it says are deleted (sent as Missing)
func (m *Walker) missing(r Result, unseen map[string]*dto.FileDto) []*dto.FileDto {
	switch {
	case !r.Complete:
		m.logger.Warn("Walk incomplete: deletions are not checked")
		return nil
	case r.Files == 0:
		m.logger.Warn("Walk found no files: deletions are not checked", l.String("path", r.Root))
		return nil
	}
	stale := make([]*dto.FileDto, 0, len(unseen))
	for _, f := range unseen {
		stale = append(stale, f)
	}
	// Read again as they are now: the rows were read at the walk's start, and the
	// chain works meanwhile — a moved file's old row may be gone already (its item
	// moved to the new path); sent as missing, it would delete that item
	ids := make([]uint, 0, len(stale))
	for _, f := range Missing(r, stale) {
		ids = append(ids, f.ID)
	}
	current, err := m.db.GetFilesByID(ids)
	if err != nil {
		m.logger.Error("Deletions: can't read the files again", l.Error(err))
		return nil
	}
	slices.SortFunc(current, func(a, b *dto.FileDto) int { return strings.Compare(a.Path, b.Path) })
	return current
}

func (m *Walker) walk(ctx context.Context, emit func(dto.ItemEntry) bool) Result {
	result := Result{Root: m.root}

	if info, err := os.Stat(m.root); err != nil || !info.IsDir() {
		m.logger.Error("Library root is not a readable directory", l.String("path", m.root), l.Error(err))
		return result
	}

	err := filepath.WalkDir(m.root, func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if path == m.root {
				return err
			}
			m.logger.Warn("Unreadable, skipped", l.String("path", path), l.Error(err))
			result.Unreadable = append(result.Unreadable, path)
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if m.skip[path] {
				return fs.SkipDir // a library's directory without its media
			}
			return nil
		}

		result.Files++
		// A file that vanished since it was listed is not seen: the gate will take it
		// as deleted
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if !emit(dto.ItemEntry{Path: path, Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime()}) {
			return ctx.Err()
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, context.Canceled) {
			m.logger.Error("Walk failed", l.String("path", m.root), l.Error(err))
		}
		return result
	}
	result.Complete = true
	return result
}

// Missing: of the rows a walk did not see (stale), the ones it says are deleted —
// only after a complete walk that found files, only under its root (another root:
// the config changed, not ours to judge), never under an unreadable directory
func Missing(r Result, stale []*dto.FileDto) []*dto.FileDto {
	if !r.Complete || r.Files == 0 {
		return nil // an empty root (an unmounted drive's mount point) must not delete the library
	}
	var gone []*dto.FileDto
	for _, f := range stale {
		if !isUnder(f.Path, r.Root) {
			continue
		}
		if slices.ContainsFunc(r.Unreadable, func(dir string) bool { return f.Path == dir || isUnder(f.Path, dir) }) {
			continue
		}
		gone = append(gone, f)
	}
	return gone
}

func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
