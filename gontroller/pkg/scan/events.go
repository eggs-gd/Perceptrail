package scan

import (
	"time"

	"perceptrail/gontroller/pkg/model/dto"
)

// What flows between the steps of the import chain (see _sb/puml/Import chain.puml)

// fileEvent: fswalker -> groups switch. One found file, or the end-of-walk marker.
type fileEvent struct {
	entry dto.ItemEntry
	done  *walkResult
}

// FileGroup is one whole asset: all its files (a main file and its sidecars,
// derivatives), or the end-of-walk marker. Groupers build it (files not stored
// yet: only the stat is set), the files gate stores it (rows of the files table,
// with GUIDs), exif turns it into a RawItem. Done is set on a grouper's marker,
// which may come together with its last group.
type FileGroup struct {
	Files []*dto.FileDto
	Done  *walkResult
}

// walkResult describes a finished walk. Deletions may be derived from it only if
// the walk was complete: a cancelled walk or an unreadable root says nothing about
// which files are gone.
type walkResult struct {
	root    string
	started time.Time
	// The walk reached the end
	complete bool
	// Files seen (before grouping and filtering)
	files int
	// Directories that could not be read: their files are not "deleted"
	unreadable []string
}
