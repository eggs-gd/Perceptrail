package providers

import (
	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

// The contract between the import and a provider's grouper: the walk's files in
// (their rows: path, stat; Changed, Gone), whole assets out; on the walk's flush,
// what the grouper still holds. A file the walk says is gone comes too: the grouper
// passes it through (an asset of its own) or makes something of it.

// Asset: one whole asset as its source describes it — all its files (the main file
// first when the source knows it, its sidecars, derivatives): the rows the walk gave
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
