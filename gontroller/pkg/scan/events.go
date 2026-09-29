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

// fileGroup: groupers -> files gate. Files that belong together (a main file and
// its sidecars, not ranked yet), or the end-of-walk marker of one grouper branch.
type fileGroup struct {
	entries []dto.ItemEntry
	done    *walkResult
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
