package dto

import "github.com/eggs-gd/perceplib/api"

// Asset: one whole asset before it is identified — all its files (the main file
// first when its source knows it, its sidecars, derivatives) as rows of the files
// table, and what its source says about it. A provider's grouper makes it; the
// import identifies it into an item (ItemDto: item == asset).
type Asset struct {
	Files []*FileDto
	// Files gone for the library: the walk found them missing, or the source says
	// so (Apple Photos: an asset trashed or hidden there). The gate has the model
	// apply it (Gone: a main file's item deleted, a sidecar's item processed again).
	Missing []*FileDto
	// Set by a source that knows the asset (Apple Photos: the asset UUID): the item's
	// GUID, and Files[0] is the main file as the source decided — it is not re-ranked.
	// "" (a plain folder): the GUID of the main file, ranked by its kind.
	Key api.GUID
	// What to show first, best first (e.g. the edit before the original); nil: the
	// import decides by itself
	Show []*FileDto
	// Metadata from the source itself (the Apple Photos DB, exiftool's tag names):
	// wins over the files' EXIF; MetaHash tells the import it changed while the files
	// did not
	Meta     api.RawExif
	MetaHash string
	// The content's fingerprint as the source knows it (Immich: the checksum), when
	// its files are not on a disk to hash; "": the import hashes the main file
	Fingerprint string
	// What the asset is (Kind*), when the source says it
	Kind string
}
