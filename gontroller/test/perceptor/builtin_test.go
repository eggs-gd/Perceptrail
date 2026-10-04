package perceptor_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/perceptor/builtin"
	"perceptrail/gontroller/pkg/perceptor/date"
	"perceptrail/gontroller/pkg/perceptor/duration"
	"perceptrail/gontroller/pkg/perceptor/size"

	"github.com/eggs-gd/perceplib/api"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// A built-in perceptor reads only the tags it declares: the core reads nothing else
// (an undeclared tag is always "")
func TestReadsDeclaredTags(t *testing.T) {
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	for perceptor, items := range map[api.Perceptor][]map[string]string{
		date.Perceptor: {
			{},
			{"MIMEType": "video/quicktime"},
			{"MIMEType": "video/quicktime", "CreateDate": "2024:01:02 10:00:00"},
			{"DateTimeOriginal": "2024:01:02 10:00:00"},
			{"DateTimeOriginal": "2024:01:02 10:00:00", "GPSLatitude": "50.45", "GPSLongitude": "30.516667"},
		},
		size.Perceptor: {
			{},
			{"ImageWidth": "4000", "ImageHeight": "3000"},
		},
		duration.Perceptor: {
			{},
		},
	} {
		built := perceptor.(builtin.Perceptor)
		for _, exif := range items {
			r := newRecorder(exif)
			if _, err := built.Decorator(logger).Decorate(r); err != nil {
				t.Fatal(err)
			}
			r.assertDeclared(t, built)
		}
	}
}

// Largest first; a section where the coarsest changed level changes (the value,
// then the finer one); ties in the incoming order; no value last under none
func TestOrderByValue(t *testing.T) {
	item := func(guid string, d float64) api.ItemDataProvider {
		return storedItem(&dto.ItemDto{Guid: guid, Duration: d})
	}
	got := builtin.OrderByValue([]api.ItemDataProvider{
		item("photo", 0), item("a", 125), item("b", 130), item("c", 61), item("c2", 61),
	}, func(it api.ItemDataProvider) float64 { return it.GetDuration() },
		func(_ api.ItemDataProvider, v float64) []string {
			return []string{fmt.Sprintf("%d min", int(v/60)), fmt.Sprintf("%d s", int(v))}
		}, "No length")

	// A new coarse section starts its finer one too (a path)
	want := []string{"b|0:2 min|1:130 s", "a|1:125 s", "c|0:1 min|1:61 s", "c2", "photo|0:No length"}
	for i, w := range want {
		s := got[i].Guid
		for _, sec := range got[i].Sections {
			s += fmt.Sprintf("|%d:%s", sec.Level, sec.Label)
		}
		if s != w {
			t.Errorf("%d: %q, want %q", i, s, w)
		}
	}
}

var _ builtin.Item = (*recorder)(nil)

// recorder: an item that remembers every tag asked of it (GetExif); Exif holds the
// values it answers with
type recorder struct {
	Exif  map[string]string
	Asked map[string]bool

	date        time.Time
	size, ratio api.Size
	duration    float64
	values      map[string]api.Values
}

func newRecorder(exif map[string]string) *recorder {
	return &recorder{Exif: exif, Asked: map[string]bool{}, values: map[string]api.Values{}}
}

func (r *recorder) GetExif(key string) string { r.Asked[key] = true; return r.Exif[key] }

func (r *recorder) GetGuid() string      { return "guid" }
func (r *recorder) GetDate() time.Time   { return r.date }
func (r *recorder) GetSize() api.Size    { return r.size }
func (r *recorder) GetRatio() api.Size   { return r.ratio }
func (r *recorder) GetDuration() float64 { return r.duration }
func (r *recorder) StoreValues(store string) (api.Values, bool) {
	v, ok := r.values[store]
	return v, ok
}
func (r *recorder) SetStoreValues(store string, v api.Values)       { r.values[store] = v }
func (r *recorder) SetDateInfo(date time.Time, source, zone string) { r.date = date }
func (r *recorder) SetDuration(seconds float64)                     { r.duration = seconds }
func (r *recorder) SetSize(size api.Size)                           { r.size = size }
func (r *recorder) SetRatio(ratio api.Size)                         { r.ratio = ratio }

// assertDeclared: every tag the perceptor asked is one it declares — the core reads
// only declared tags, an undeclared one is always ""
func (r *recorder) assertDeclared(t *testing.T, p api.ExifTagger) {
	t.Helper()
	declared := p.ExifTags()
	for tag := range r.Asked {
		if !slices.Contains(declared, tag) {
			t.Errorf("reads %s, which it does not declare (ExifTags)", tag)
		}
	}
}

// stored: a stored item as a perceptor's view reads it (Order, Info), the way the
// web hands it over: the item and its perceptors' values
func storedItem(it *dto.ItemDto) api.ItemDataProvider { return &stored{ItemDto: it} }

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
