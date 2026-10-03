package providers

import (
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// The contract between the import's discover stage and a provider's grouper: found
// files in, whole assets out, the end-of-walk marker through.

// Found: one found file (its path and stat), or the end-of-walk marker (Done)
type Found struct {
	Entry dto.ItemEntry
	Done  *Walk
}

// Walk describes a finished walk; it rides in the end-of-walk marker. Deletions may
// be derived from it only if the walk was complete: a cancelled walk or an
// unreadable root says nothing about which files are gone.
type Walk struct {
	Root    string
	Started time.Time
	// The walk reached the end
	Complete bool
	// Files seen (before grouping and filtering)
	Files int
	// Directories that could not be read: their files are not "deleted"
	Unreadable []string
}

// Asset: one whole asset as its source describes it — all its files (the main file
// first when the source knows it, its sidecars, derivatives), with their stat only
type Asset struct {
	Files []*dto.FileDto
	// Set by a source that knows the asset (Apple Photos: the asset UUID): the item's
	// GUID, and Files[0] is the main file as the source decided — it is not re-ranked.
	// "" (a plain folder): the GUID of the main file, ranked by its kind.
	Key string
	// What to show first, best first (e.g. the edit before the original); nil: the
	// import decides by itself
	Show []*dto.FileDto
	// Metadata from the source itself (the Apple Photos DB, exiftool's tag names):
	// wins over the files' EXIF; MetaHash tells the import it changed while the files
	// did not
	Meta     api.RawExif
	MetaHash string
	// What the asset is (dto.Kind*), when the source says it
	Kind string
}

// Group: what a grouper sends — a complete asset, and/or its end-of-walk marker
// (Done: the grouper has flushed; it may come with its last asset)
type Group struct {
	Asset
	Done *Walk
	// With the marker: files the grouper saw but held back (their asset did not
	// complete in this walk) — not "gone" for the deletions
	Held []string
}
