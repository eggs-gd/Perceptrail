package config

import (
	"slices"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestResolve(t *testing.T) {
	f := file{
		Path:     "photos",
		Plugins:  []string{"../.build/plugins/geo.so", "/abs/other.so"},
		Exiftool: "exiftool",
	}
	if err := f.resolve("/srv/gontroller/.var"); err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"Path":         {f.Path, "/srv/gontroller/.var/photos"},
		"relative .so": {f.Plugins[0], "/srv/gontroller/.build/plugins/geo.so"},
		"absolute .so": {f.Plugins[1], "/abs/other.so"},
		"DataDir":      {f.DataDir, "/srv/gontroller/.var"},
		"Database":     {f.Database.Name, "/srv/gontroller/.var/media_library.db"},
		"Driver":       {f.Database.Driver, "sqlite"},
		"Exiftool":     {f.Exiftool, "exiftool"}, // bare name: looked up in PATH
	}
	for name, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", name, v[0], v[1])
		}
	}

	f = file{DataDir: "data", Exiftool: "tools/exiftool"}
	f.Database.Name = "/db/photos.db"
	if err := f.resolve("/cfg"); err != nil {
		t.Fatal(err)
	}
	if f.DataDir != "/cfg/data" || f.Exiftool != "/cfg/tools/exiftool" || f.Database.Name != "/db/photos.db" {
		t.Errorf("got DataDir %q, Exiftool %q, Database.Name %q", f.DataDir, f.Exiftool, f.Database.Name)
	}

	// Server drivers: Name is a database name, not a file
	f = file{}
	f.Database.Driver, f.Database.Name = "postgres", "perceptrail"
	if err := f.resolve("/cfg"); err != nil {
		t.Fatal(err)
	}
	if f.Database.Name != "perceptrail" {
		t.Errorf("postgres Name = %q", f.Database.Name)
	}
}

// Rescan: a duration in the file, a minute when unset
func TestRescan(t *testing.T) {
	var f file
	if err := yaml.Unmarshal([]byte("rescan: 30s\n"), &f); err != nil || f.Rescan != 30*time.Second {
		t.Errorf("rescan = %v, %v", f.Rescan, err)
	}
	f = file{}
	if err := f.resolve(t.TempDir()); err != nil || f.Rescan != time.Minute {
		t.Errorf("default rescan = %v, %v", f.Rescan, err)
	}
}

// Release unless the config says debug: an unknown mode is not a debug one
func TestMode(t *testing.T) {
	for mode, want := range map[string]string{"": ModeRelease, "release": ModeRelease, "debug": ModeDebug, "verbose": ModeRelease} {
		c := Config{file: file{Mode: mode}}
		if err := c.file.resolve(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if c.Mode() != want || c.Debug() != (want == ModeDebug) {
			t.Errorf("%q: %q, want %q", mode, c.Mode(), want)
		}
	}
}

func TestServer(t *testing.T) {
	s := Server{}
	if err := s.resolve(); err != nil || s.Addr() != ":1323" || s.AllowedOrigins[0] != "*" {
		t.Errorf("defaults: %+v, %v", s, err)
	}
	s = Server{Host: "::1", Port: 8080, AllowedOrigins: []string{"http://localhost:5173"}}
	if err := s.resolve(); err != nil || s.Addr() != "[::1]:8080" || s.AllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("explicit: %+v, %v", s, err)
	}
	if err := (&Server{Port: 70000}).resolve(); err == nil {
		t.Error("port 70000: want an error")
	}
}

func TestPerceptors(t *testing.T) {
	var s Perceptors
	if err := yaml.Unmarshal([]byte("exif_size:\n  client: false\nexif_geo:\n  enabled: false\n  client: true\n"), &s); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name            string
		enabled, client bool
	}{
		{"exif_date", true, true}, // not listed: defaults
		{"exif_size", true, false},
		{"exif_geo", false, false}, // not run: not shown either
	} {
		if s.Enabled(c.name) != c.enabled || s.Client(c.name) != c.client {
			t.Errorf("%s: enabled %v client %v, want %v %v", c.name, s.Enabled(c.name), s.Client(c.name), c.enabled, c.client)
		}
	}
}

// A provider not listed is enabled; enabled: false takes it out of the chain
func TestProviders(t *testing.T) {
	var f file
	if err := yaml.Unmarshal([]byte("providers:\n  apple:\n    enabled: false\n"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Providers.Enabled("apple") || !f.Providers.Enabled("immich") || !(Providers(nil)).Enabled("apple") {
		t.Errorf("providers %+v", f.Providers)
	}
}

// Render sizes: sorted, a repeated one once (two renditions of one size would clash)
func TestRenderSizes(t *testing.T) {
	r := Render{Sizes: []int{1600, 400, 400}}
	if err := r.resolve(func(p string) string { return p }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Sizes, []int{400, 1600}) {
		t.Errorf("sizes %v, want [400 1600]", r.Sizes)
	}
}
