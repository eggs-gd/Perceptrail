// Package exif_coretest: helpers for testing perceptors.
package exif_coretest

import (
	"slices"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
)

var _ exif_core.RawItemRW = (*Recorder)(nil)

// Recorder: an item that remembers every tag asked of it (GetExif); Exif holds the
// values it answers with
type Recorder struct {
	Exif  map[string]string
	Asked map[string]bool

	date        time.Time
	size, ratio api.Size
	duration    float64
	values      map[string]api.Values
}

func NewRecorder(exif map[string]string) *Recorder {
	return &Recorder{Exif: exif, Asked: map[string]bool{}, values: map[string]api.Values{}}
}

func (r *Recorder) GetExif(key string) string { r.Asked[key] = true; return r.Exif[key] }

func (r *Recorder) GetGuid() string      { return "guid" }
func (r *Recorder) GetDate() time.Time   { return r.date }
func (r *Recorder) GetSize() api.Size    { return r.size }
func (r *Recorder) GetRatio() api.Size   { return r.ratio }
func (r *Recorder) GetDuration() float64 { return r.duration }
func (r *Recorder) StoreValues(store string) (api.Values, bool) {
	v, ok := r.values[store]
	return v, ok
}
func (r *Recorder) SetStoreValues(store string, v api.Values)       { r.values[store] = v }
func (r *Recorder) SetDate(date time.Time)                          { r.date = date }
func (r *Recorder) SetDateInfo(date time.Time, source, zone string) { r.date = date }
func (r *Recorder) SetDuration(seconds float64)                     { r.duration = seconds }
func (r *Recorder) SetSize(size api.Size)                           { r.size = size }
func (r *Recorder) SetRatio(ratio api.Size)                         { r.ratio = ratio }

// AssertDeclared: every tag the perceptor asked is one it declares — the core reads
// only declared tags, an undeclared one is always ""
func (r *Recorder) AssertDeclared(t *testing.T, p api.ExifTagger) {
	t.Helper()
	declared := p.ExifTags()
	for tag := range r.Asked {
		if !slices.Contains(declared, tag) {
			t.Errorf("reads %s, which it does not declare (ExifTags)", tag)
		}
	}
}
