package group

import (
	"strings"
	"testing"

	"perceptrail/gontroller/internal/library/folder"
	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"
)

// claimsJPEG: a provider that claims .jpg files (only Claims is asked here)
type claimsJPEG struct{ provider.Provider }

func (claimsJPEG) Claims(path string) bool { return strings.HasSuffix(path, ".jpg") }

// A file to the first provider that claims it, the rest to the plain folder (the
// walk's flush reaches every grouper: the switch's outputs close when it returns)
func TestSwitch(t *testing.T) {
	s := Switch{Providers: []provider.Provider{claimsJPEG{}, folder.New()}}
	if got, _ := s.Switch(&dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/a.jpg"}}); got != 0 {
		t.Errorf("a claimed file goes to its provider's grouper, got %d", got)
	}
	if got, _ := s.Switch(&dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/a.mov"}}); got != 1 {
		t.Errorf("an unclaimed file goes to the plain folder, got %d", got)
	}
}
