package api

import (
	"testing"
	"time"
)

type carrier struct{ v map[string]Values }

func (c *carrier) StoreValues(store string) (Values, bool) { v, ok := c.v[store]; return v, ok }
func (c *carrier) SetStoreValues(store string, v Values)   { c.v[store] = v }

type testItem struct{ carrier }

func (testItem) GetGUID() GUID        { return "g" }
func (testItem) GetDate() time.Time   { return time.Time{} }
func (testItem) GetSize() Size        { return Size{} }
func (testItem) GetRatio() Size       { return Size{} }
func (testItem) GetDuration() float64 { return 0 }

type sample struct {
	Lat      float64
	GPSCount int64
	Place    string
	Seen     bool
	At       time.Time
	Embed    []float32 `perceptor:"dim=3"`
	Custom   string    `perceptor:"name=label"`
	hidden   int
}

func TestStore(t *testing.T) {
	s := NewStore[sample]("test", 2)
	sc := s.Schema()
	want := []Field{{"lat", KindFloat, 0}, {"gps_count", KindInt, 0}, {"place", KindText, 0}, {"seen", KindBool, 0},
		{"at", KindTime, 0}, {"embed", KindVector, 3}, {"label", KindText, 0}}
	if sc.Store != "test" || sc.Version != 2 || len(sc.Fields) != len(want) {
		t.Fatalf("schema %+v", sc)
	}
	for i, f := range want {
		if sc.Fields[i] != f {
			t.Errorf("field %d: %+v, want %+v", i, sc.Fields[i], f)
		}
	}

	it := &testItem{carrier{map[string]Values{}}}
	if _, ok := s.Get(it); ok {
		t.Error("a value before Put")
	}
	in := sample{Lat: 50.4, GPSCount: 3, Place: "Kyiv", Seen: true, At: time.Unix(100, 0).UTC(), Embed: []float32{1, 2, 3}, Custom: "x"}
	s.Put(it, in)
	got, ok := s.Get(it)
	if !ok || got.Lat != 50.4 || got.GPSCount != 3 || got.Place != "Kyiv" || !got.Seen || !got.At.Equal(in.At) ||
		len(got.Embed) != 3 || got.Embed[2] != 3 || got.Custom != "x" {
		t.Errorf("round trip %+v", got)
	}
}

func TestStorePanics(t *testing.T) {
	for name, f := range map[string]func(){
		"not a struct":   func() { NewStore[int]("x", 1) },
		"vector, no dim": func() { NewStore[struct{ V []float32 }]("x", 1) },
		"unsupported":    func() { NewStore[struct{ M map[string]int }]("x", 1) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			f()
		}()
	}
}
