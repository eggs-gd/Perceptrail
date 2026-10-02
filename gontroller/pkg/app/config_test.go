package app

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestConfigResolve(t *testing.T) {
	c := Config{
		Path:     "photos",
		Plugins:  []string{"../.build/plugins/geo.so", "/abs/other.so"},
		Exiftool: "exiftool",
	}
	c.resolve("/srv/gontroller/.var")

	checks := map[string][2]string{
		"Path":         {c.Path, "/srv/gontroller/.var/photos"},
		"relative .so": {c.Plugins[0], "/srv/gontroller/.build/plugins/geo.so"},
		"absolute .so": {c.Plugins[1], "/abs/other.so"},
		"DataDir":      {c.DataDir, "/srv/gontroller/.var"},
		"Database":     {c.Database.Name, "/srv/gontroller/.var/media_library.db"},
		"Driver":       {c.Database.Driver, "sqlite"},
		"Exiftool":     {c.Exiftool, "exiftool"}, // bare name: looked up in PATH
	}
	for name, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", name, v[0], v[1])
		}
	}

	c = Config{DataDir: "data", Exiftool: "tools/exiftool"}
	c.Database.Name = "/db/photos.db"
	c.resolve("/cfg")
	if c.DataDir != "/cfg/data" || c.Exiftool != "/cfg/tools/exiftool" || c.Database.Name != "/db/photos.db" {
		t.Errorf("got DataDir %q, Exiftool %q, Database.Name %q", c.DataDir, c.Exiftool, c.Database.Name)
	}

	// Server drivers: Name is a database name, not a file
	c = Config{}
	c.Database.Driver, c.Database.Name = "postgres", "perceptrail"
	c.resolve("/cfg")
	if c.Database.Name != "perceptrail" {
		t.Errorf("postgres Name = %q", c.Database.Name)
	}
}

func TestConfigRescanDuration(t *testing.T) {
	var c Config
	if err := yaml.Unmarshal([]byte("rescan: 30s\n"), &c); err != nil || c.Rescan != 30*time.Second {
		t.Errorf("rescan = %v, %v", c.Rescan, err)
	}
}

// Release unless the config says debug: an unknown mode is not a debug one
func TestConfigMode(t *testing.T) {
	for mode, want := range map[string]string{"": ModeRelease, "release": ModeRelease, "debug": ModeDebug, "verbose": ModeRelease} {
		c := Config{Mode: mode}
		c.resolve(t.TempDir())
		if c.Mode != want || c.Debug() != (want == ModeDebug) {
			t.Errorf("%q: %q, want %q", mode, c.Mode, want)
		}
	}
}

// A provider not listed is enabled; enabled: false takes it out of the chain
func TestProvidersEnabled(t *testing.T) {
	var c Config
	if err := yaml.Unmarshal([]byte("providers:\n  apple:\n    enabled: false\n"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Providers.Enabled("apple") || !c.Providers.Enabled("immich") || !(Providers(nil)).Enabled("apple") {
		t.Errorf("providers %+v", c.Providers)
	}
}
