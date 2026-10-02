package groups

import (
	"testing"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"
	"perceptrail/gontroller/pkg/scan/flow"
)

// claimsJPEG: a provider that claims .jpg files (only Claims is asked here)
type claimsJPEG struct{ providers.Provider }

func (claimsJPEG) Claims(path string) bool { return len(path) > 4 && path[len(path)-4:] == ".jpg" }

// A provider's step: its files to its grouper, the rest on; the marker to both
func TestClaim(t *testing.T) {
	c := Claim{Provider: claimsJPEG{}}
	got, _ := c.Switch(flow.FileEvent{Entry: dto.ItemEntry{Path: "/lib/a.jpg"}})
	if _, ok := got[Own]; !ok || len(got) != 1 {
		t.Errorf("a claimed file goes to the provider's grouper, got %v", got)
	}
	got, _ = c.Switch(flow.FileEvent{Entry: dto.ItemEntry{Path: "/lib/a.mov"}})
	if _, ok := got[Next]; !ok || len(got) != 1 {
		t.Errorf("an unclaimed file goes on, got %v", got)
	}
	got, _ = c.Switch(flow.FileEvent{Done: &flow.WalkResult{}})
	if len(got) != 2 {
		t.Errorf("the marker must reach the grouper and the next step, got %d", len(got))
	}
	if Branches(nil) != 1 || Branches([]providers.Provider{c.Provider}) != 2 {
		t.Error("branches: one per provider and the generic grouper")
	}
}
