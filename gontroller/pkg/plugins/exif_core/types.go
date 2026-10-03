package exif_core

import (
	"time"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type RawItemRW interface {
	api.RawItemR
	api.ItemDataEditor
	// SetDateInfo stores the date with its offset (date's zone), the tag it came
	// from and how the zone was found. Core only: external plugins read GetDate(),
	// which returns the date in that zone.
	SetDateInfo(date time.Time, source, zone string)
	// SetDuration stores a video's length, seconds. Core only.
	SetDuration(seconds float64)
}

type ExifCorePerceptor interface {
	api.Perceptor
	api.ExifTagger
	// Decorator: the perceptor's logic over one item (it writes into it); the core
	// runs it as a step
	Decorator(logger *l.Logger) chain.Decorator[RawItemRW, RawItemRW]
}
