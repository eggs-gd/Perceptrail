package api

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Perceptor data: a perceptor declares what it keeps per item as a Go struct and
// gets a typed Store of it; the host creates, migrates and maintains the storage
// (SQLite: a file per perceptor; Postgres: a schema per perceptor), commits the
// values with the item and loads them for Order. The plugin never sees SQL.
//
//	type Location struct {
//	    Lat float64
//	    Lon float64
//	}
//
//	var Places = api.NewStore[Location]("geo", 1)
//
//	func (p *geoPerceptor) Schema() api.Schema { return Places.Schema() }
//	Places.Put(in, Location{Lat: lat, Lon: lon}) // in the processor
//	loc, ok := Places.Get(it)                    // in Order

// Kind of a stored field, from the Go type of the struct field
type Kind int

const (
	KindFloat  Kind = iota + 1 // float64
	KindInt                    // int64, int
	KindText                   // string
	KindBool                   // bool
	KindTime                   // time.Time
	KindVector                 // []float32; its size from the tag: `perceptor:"dim=512"`
)

// Field: one stored value of a store
type Field struct {
	Name string // snake_case of the Go field, or the tag's name=
	Kind Kind
	Dim  int // KindVector only
}

// Schema: what a perceptor keeps; the zero Schema keeps nothing (Store == "")
type Schema struct {
	Store   string
	Version int // a change rebuilds the store and processes the items again
	Fields  []Field
}

// Values: one item's values of one store, by field name. The exchange between
// Store[T] and the host — plugins use Put / Get.
type Values map[string]any

// ValueCarrier: an item carries the values of the stores (the host implements it)
type ValueCarrier interface {
	StoreValues(store string) (Values, bool)
	SetStoreValues(store string, v Values)
}

// Store: a perceptor's typed data. Create it once (a package variable): the schema
// is read from T by reflection, and an unsupported field panics at plugin load.
type Store[T any] struct {
	schema Schema
	index  []int // T's field per schema field
}

func NewStore[T any](name string, version int) *Store[T] {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("perceptor store %q: %s is not a struct", name, t))
	}
	s := &Store[T]{schema: Schema{Store: name, Version: version}}
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		f := Field{Name: snake(sf.Name)}
		for _, opt := range strings.Split(sf.Tag.Get("perceptor"), ",") {
			k, v, _ := strings.Cut(strings.TrimSpace(opt), "=")
			switch k {
			case "name":
				f.Name = v
			case "dim":
				f.Dim, _ = strconv.Atoi(v)
			}
		}
		switch {
		case sf.Type == reflect.TypeFor[time.Time]():
			f.Kind = KindTime
		case sf.Type == reflect.TypeFor[[]float32]():
			if f.Dim <= 0 {
				panic(fmt.Sprintf("perceptor store %q: vector %s needs `perceptor:\"dim=N\"`", name, sf.Name))
			}
			f.Kind = KindVector
		default:
			switch sf.Type.Kind() {
			case reflect.Float64, reflect.Float32:
				f.Kind = KindFloat
			case reflect.Int, reflect.Int64, reflect.Int32:
				f.Kind = KindInt
			case reflect.String:
				f.Kind = KindText
			case reflect.Bool:
				f.Kind = KindBool
			default:
				panic(fmt.Sprintf("perceptor store %q: field %s: unsupported type %s", name, sf.Name, sf.Type))
			}
		}
		s.schema.Fields = append(s.schema.Fields, f)
		s.index = append(s.index, i)
	}
	return s
}

func (s *Store[T]) Schema() Schema { return s.schema }

// Put: v is this item's value (the host commits it with the item)
func (s *Store[T]) Put(item ItemDataProvider, v T) {
	rv := reflect.ValueOf(v)
	vals := make(Values, len(s.index))
	for i, f := range s.schema.Fields {
		vals[f.Name] = rv.Field(s.index[i]).Interface()
	}
	item.SetStoreValues(s.schema.Store, vals)
}

// Get: the item's value; false if the item has none (not processed, or nothing found)
func (s *Store[T]) Get(item ItemDataProvider) (T, bool) {
	var out T
	vals, ok := item.StoreValues(s.schema.Store)
	if !ok {
		return out, false
	}
	rv := reflect.ValueOf(&out).Elem()
	for i, f := range s.schema.Fields {
		v, ok := vals[f.Name]
		if !ok || v == nil {
			continue
		}
		field := rv.Field(s.index[i])
		val := reflect.ValueOf(v)
		if !val.Type().ConvertibleTo(field.Type()) {
			continue
		}
		field.Set(val.Convert(field.Type()))
	}
	return out, true
}

// snake: "GPSLat" → "gps_lat", "Lat" → "lat"
func snake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
