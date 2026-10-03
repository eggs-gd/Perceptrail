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

// A file to the first provider that claims it, the rest to the plain folder (the
// walk's flush reaches every grouper: the chain's Route sends it to every output)
func TestSwitch(t *testing.T) {
	s := Switch{Providers: []providers.Provider{claimsJPEG{}, folder.New()}}
	if got, _ := s.Route(dto.ItemEntry{Path: "/lib/a.jpg"}); got != 0 {
		t.Errorf("a claimed file goes to its provider's grouper, got %d", got)
	}
	if got, _ := s.Route(dto.ItemEntry{Path: "/lib/a.mov"}); got != 1 {
		t.Errorf("an unclaimed file goes to the plain folder, got %d", got)
	}
}
