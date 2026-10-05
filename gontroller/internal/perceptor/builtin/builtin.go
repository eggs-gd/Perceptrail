package builtin

import (
	"time"

	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
)

// Item: the item as the core perceptors see it — they write into it; external
// plugins only read (api.RawItemR)
type Item interface {
	api.RawItemR
	// SetSize / SetRatio: the oriented pixel size and its ratio. Core only.
	SetSize(size api.Size)
	SetRatio(ratio api.Size)
	// SetDateInfo stores the date with its offset (date's zone), the tag it came
	// from and how the zone was found. Core only: external plugins read GetDate(),
	// which returns the date in that zone.
	SetDateInfo(date time.Time, source, zone string)
	// SetDuration stores a video's length, seconds. Core only.
	SetDuration(seconds float64)
}

type Perceptor interface {
	api.Perceptor
	api.ExifTagger
	// Decorator: the perceptor's logic over one item (it writes into it); the core
	// runs it as a step
	Decorator(logger *l.Logger) chain.Decorator[Item, Item]
}
