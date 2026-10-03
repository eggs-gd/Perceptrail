package providers

import (
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// The contract between the import and a provider's grouper: found files in (their
// path and stat, dto.ItemEntry), whole assets out; on the walk's flush, what the
// grouper still holds.

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
// saw but held back (their asset did not complete in this walk: they are there, the
// deletions must not take them as gone)
type Group struct {
	Asset
	Held []string
	// Requested: the importer asks for this asset again on demand (the library made
	// a file of it local): processed even if nothing changed
	Requested bool
}
