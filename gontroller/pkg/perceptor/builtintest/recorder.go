// Package builtintest: helpers for testing perceptors.
package builtintest

import (
	"slices"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
)

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

// Stored: a stored item as a perceptor's view reads it (Order, Info), the way the
// web hands it over: the item and its perceptors' values
func Stored(it *dto.ItemDto) api.ItemDataProvider { return &stored{ItemDto: it} }

type stored struct {
	*dto.ItemDto
	values map[string]api.Values
}

func (s *stored) GetGuid() string      { return s.Guid }
func (s *stored) GetSize() api.Size    { return s.Size }
func (s *stored) GetRatio() api.Size   { return s.Ratio }
func (s *stored) GetDuration() float64 { return s.Duration }

func (s *stored) StoreValues(store string) (api.Values, bool) {
	v, ok := s.values[store]
	return v, ok
}

func (s *stored) SetStoreValues(store string, v api.Values) {
	if s.values == nil {
		s.values = map[string]api.Values{}
	}
	s.values[store] = v
}
