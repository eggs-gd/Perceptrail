package app

import "testing"

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
		"DatabasePath": {c.DatabasePath(), "/srv/gontroller/.var/media_library.db"},
		"Exiftool":     {c.Exiftool, "exiftool"}, // bare name: looked up in PATH
		"Addr":         {c.Addr(), ":1323"},
	}
	for name, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", name, v[0], v[1])
		}
	}

	c = Config{DataDir: "data", Exiftool: "tools/exiftool"}
	c.Server.Host, c.Server.Port = "127.0.0.1", 8080
	c.resolve("/cfg")
	if c.DataDir != "/cfg/data" || c.Exiftool != "/cfg/tools/exiftool" || c.Addr() != "127.0.0.1:8080" {
		t.Errorf("got DataDir %q, Exiftool %q, Addr %q", c.DataDir, c.Exiftool, c.Addr())
	}
}
