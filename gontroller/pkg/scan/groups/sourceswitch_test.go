package groups

import (
	"testing"

	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/scan/flow"
)

func TestSourceSwitch(t *testing.T) {
	lib := flow.FileEvent{Entry: dto.ItemEntry{Path: "/Pictures/Photos Library.photoslibrary/originals/A/x.heic"}}
	got, _ := SourceSwitch{}.Switch(lib)
	if _, ok := got[BranchApple]; !ok || len(got) != 1 {
		t.Errorf("a Photos library goes to the apple grouper, got %v", got)
	}
	plain, _ := SourceSwitch{}.Switch(flow.FileEvent{Entry: dto.ItemEntry{Path: "/Pictures/a.jpg"}})
	if _, ok := plain[BranchGeneric]; !ok || len(plain) != 1 {
		t.Errorf("a plain folder goes to generic, got %v", plain)
	}
	marker, _ := SourceSwitch{}.Switch(flow.FileEvent{Done: &flow.WalkResult{}})
	if len(marker) != Branches {
		t.Errorf("the marker must reach every grouper, got %d", len(marker))
	}
	if !inPhotosLibrary(lib.Entry.Path) || inPhotosLibrary("/Pictures/a.jpg") {
		t.Error("inPhotosLibrary")
	}
}
