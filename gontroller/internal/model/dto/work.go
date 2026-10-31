package dto

import (
	"time"

	"github.com/eggs-gd/perceplib/api"
)

// WorkDto: one kind of work for an item — a slug ("render"; later a pixel
// perceptor), what it was last done or tried with, and its state in the queue.
// What is needed is derived, never recorded: an item needs the work when it has no
// row, or its row was done with another version or for another input. Times are unix
// seconds (SQLite keeps times as text, and text compares lie).
type WorkDto struct {
	GUID       api.GUID `gorm:"primaryKey"`
	Slug       string   `gorm:"primaryKey"`
	Version    string   // what the result is: render's sizes, format and quality; a perceptor's version
	Input      string   // the item's fingerprint it was done (or tried) for
	DoneAt     int64    // 0: not done
	Attempts   int      // failures in a row with this version and input
	NextTry    int64    // after a failure: not before
	Error      string   // the last failure, kept for the info panel
	LeaseUntil int64    // taken into work until then; a crash lets it expire
	Lease      int64    // the taker's token: its result counts only while it holds the row
}

// RenditionDto: one rendition of an item — a size in a format; a new render of the
// item replaces them all
type RenditionDto struct {
	GUID   api.GUID `gorm:"primaryKey"`
	Size   int      `gorm:"primaryKey"`    // the long side asked for, px
	Format string   `gorm:"primaryKey"`    // the file's extension: webp, jpg, mp4…
	Role   string   `gorm:"default:still"` // what it is to the asset: still or motion (Role*)
	W, H   int      // pixels; 0: unknown
	Bytes  int64
	Path   string // relative to the cache: r/<ab>/<cd>/<guid>/<size>.<format>
}

// Taken: an item taken into a slug's work, and the token its result must carry
type Taken struct {
	GUID  api.GUID
	Lease int64
}

// WorkDone: a slug's work done for an item — under which lease, with what, for which
// input, what it made
type WorkDone struct {
	Slug       string
	GUID       api.GUID
	Lease      int64 // from Taken; 0: never taken (nothing to render)
	Version    string
	Input      string
	Renditions []RenditionDto
}

// WorkFailed: a slug's work failed for an item
type WorkFailed struct {
	Slug    string
	GUID    api.GUID
	Lease   int64 // from Taken
	Version string
	Input   string
	Err     string
}

// Cursor: where a pass over the items due for some work is; the zero value starts
// one (the model's Due moves it)
type Cursor struct {
	Phase int // 0: Waiting items; 1: the shown ones (Visible, Ready); 2: the pass is over
	Date  time.Time
	ID    uint // the last item of the page; 0: the phase starts
}

func (WorkDto) TableName() string { return "work" }

func (RenditionDto) TableName() string { return "renditions" }

// Over: the pass has no more pages
func (c Cursor) Over() bool { return c.Phase > 1 }
