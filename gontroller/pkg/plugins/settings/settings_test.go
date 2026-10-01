package settings

import (
	"testing"

	"gopkg.in/yaml.v3"
)

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
