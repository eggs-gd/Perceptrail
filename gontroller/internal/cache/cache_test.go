package cache

import (
	"path/filepath"
	"testing"

	"github.com/eggs-gd/perceplib/api"
)

// Two levels by the GUID's first characters, lower case; a GUID too short for them
// still gets a directory of its own
func TestItemDir(t *testing.T) {
	for guid, want := range map[string]string{
		"9d3611a6-08aa-4999-89b8-f2a3e24f7bed": "r/9d/36/9d3611a6-08aa-4999-89b8-f2a3e24f7bed",
		"0F0ADCDD-A876-45C5-8302-FC7E9E0419DF": "r/0f/0a/0F0ADCDD-A876-45C5-8302-FC7E9E0419DF",
		"ab":                                   "r/_/ab",
	} {
		if got := ItemDir("r", guidOf(guid)); got != filepath.FromSlash(want) {
			t.Errorf("%s: %s, want %s", guid, got, want)
		}
	}
}

func guidOf(s string) api.GUID { return api.GUID(s) }
