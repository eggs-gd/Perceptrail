package model

import (
	"perceptrail/gontroller/internal/config"
	"slices"
	"testing"
	"time"

	"github.com/eggs-gd/perceplib/api"
)

type storedSample struct {
	Lat   float64
	Count int64
	Name  string
	Seen  bool
	At    time.Time
	Embed []float32 `perceptor:"dim=2"`
}

func TestPerceptorStore(t *testing.T) {
	dir := t.TempDir()
	schema := api.NewStore[storedSample]("sample", 1).Schema()
	st, err := OpenPerceptorStore(config.DriverSQLite, dir, schema)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	if err := st.Save("a", api.Values{"lat": 50.4, "count": int64(3), "name": "Kyiv", "seen": true, "at": at, "embed": []float32{0.5, -1}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save("b", nil); err != nil { // processed, nothing found
		t.Fatal(err)
	}

	got, err := st.Load([]api.GUID{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("loaded %v, want only a (b has nothing, c is unknown)", got)
	}
	v := got["a"]
	if v["lat"] != 50.4 || v["count"] != int64(3) || v["name"] != "Kyiv" || v["seen"] != true ||
		!v["at"].(time.Time).Equal(at) || v["embed"].([]float32)[1] != -1 {
		t.Errorf("a = %v", v)
	}

	// Gone items: their rows go (both processed: with a value or without)
	if guids, err := st.GUIDs(); err != nil || len(guids) != 2 {
		t.Errorf("guids %v %v, want both rows", guids, err)
	}
	if n, err := st.Prune(func(g api.GUID) bool { return g == "a" }); err != nil || n != 1 {
		t.Errorf("prune: %d %v, want 1", n, err)
	}
	st.Close()

	// The same schema keeps the values; a new version drops them
	st, _ = OpenPerceptorStore(config.DriverSQLite, dir, schema)
	if guids, _ := st.GUIDs(); !slices.Equal(guids, []api.GUID{"a"}) {
		t.Errorf("reopened: %v, want a", guids)
	}
	st.Close()
	schema.Version = 2
	st, err = OpenPerceptorStore(config.DriverSQLite, dir, schema)
	if err != nil {
		t.Fatal(err)
	}
	if guids, _ := st.GUIDs(); len(guids) != 0 {
		t.Error("a new schema version kept the old values")
	}
	st.Close()

	if _, err := OpenPerceptorStore(config.DriverPostgres, dir, schema); err == nil {
		t.Error("postgres: want not implemented")
	}
}
