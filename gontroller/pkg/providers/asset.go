package providers

import (
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// The contract between the import's discover stage and a provider's grouper: found
// files in, whole assets out; on the walk's flush, what the grouper still holds.

// Found: one found file (its path and stat). The end of a walk is not a value: the
// chain flushes, and a grouper gives what it holds (chain.Flusher).
type Found struct {
	Entry dto.ItemEntry
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

// Group: what a grouper sends — a complete asset; on the walk's flush, the files it
// saw but held back (their asset did not complete in this walk: not "gone" for the
// deletions)
type Group struct {
	Asset
	Held []string
}
