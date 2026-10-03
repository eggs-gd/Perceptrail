package group

import (
	"strings"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/providers/folder"
)

// claimsJPEG: a provider that claims .jpg files (only Claims is asked here)
type claimsJPEG struct{ providers.Provider }

func (claimsJPEG) Claims(path string) bool { return strings.HasSuffix(path, ".jpg") }

// A file to the first provider that claims it, the rest to the plain folder; the
// marker to every grouper
func TestSwitch(t *testing.T) {
	s := Switch{Providers: []providers.Provider{claimsJPEG{}, folder.New()}}
	got, _ := s.Switch(providers.Found{Entry: dto.ItemEntry{Path: "/lib/a.jpg"}})
	if _, ok := got[0]; !ok || len(got) != 1 {
		t.Errorf("a claimed file goes to its provider's grouper, got %v", got)
	}
	got, _ = s.Switch(providers.Found{Entry: dto.ItemEntry{Path: "/lib/a.mov"}})
	if _, ok := got[1]; !ok || len(got) != 1 {
		t.Errorf("an unclaimed file goes to the plain folder, got %v", got)
	}
	got, _ = s.Switch(providers.Found{Done: &providers.Walk{}})
	if len(got) != 2 {
		t.Errorf("the marker must reach every grouper, got %d", len(got))
	}
}
