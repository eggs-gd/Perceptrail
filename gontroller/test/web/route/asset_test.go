package route_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"perceptrail/gontroller/internal/model/dto"
)

// A file is served only as a file of its own asset: never another asset's, never a
// path
func TestAssetFileOnlyOfItsAsset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.jpg")
	os.WriteFile(path, []byte("jpeg"), 0o644)
	f, err := testDB.CreateFile(dto.ItemEntry{Path: path, Name: "a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	f.LinkToItem("MINE")
	if _, err := testDB.UpdateFile(f); err != nil {
		t.Fatal(err)
	}

	e := server(nil)
	code := func(guid, name string) int { return get(e, "/assets/"+guid+"/"+url.PathEscape(name)).Code }
	id := func() string { return strconv.FormatUint(uint64(f.ID), 10) }
	if code := code("MINE", id()); code != http.StatusOK {
		t.Errorf("own file: %d", code)
	}
	if code := code("OTHER", id()); code != http.StatusNotFound {
		t.Errorf("another asset's file: %d, want 404", code)
	}
	if code := code("MINE", "../../etc/passwd"); code != http.StatusNotFound {
		t.Errorf("a path: %d, want 404", code)
	}
}
