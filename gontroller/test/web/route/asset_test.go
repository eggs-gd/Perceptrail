package route_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"
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

// Our renditions are stills of the asset (with their widths: the client's srcset) and
// are served by name — only the ones listed for that item
func TestRenditionsInTheAsset(t *testing.T) {
	item, err := testDB.UpdateItem(&dto.ItemDto{GUID: "R1", State: dto.Visible, HashShort: "h"})
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join("r", "R1", "v1", "400.webp")
	os.MkdirAll(filepath.Join(testCache, filepath.Dir(rel)), 0o755)
	os.WriteFile(filepath.Join(testCache, rel), []byte("webp"), 0o644)
	taken, _ := testDB.Take("render", []api.GUID{item.GUID})
	done, err := testDB.Finish(dto.WorkDone{Slug: "render", GUID: item.GUID, Lease: taken[0].Lease, Version: "v1", Input: "h",
		Renditions: []dto.RenditionDto{{GUID: item.GUID, Version: "v1", Size: 400, Format: "webp", W: 400, H: 300, Path: rel}}})
	if err != nil || !done {
		t.Fatalf("finish: %v %v", done, err)
	}

	e := server(nil)
	var stills []struct {
		URL  string `json:"url"`
		Mime string `json:"mime"`
		W    int    `json:"w"`
	}
	sc := bufio.NewScanner(get(e, "/items").Body)
	for sc.Scan() {
		var line struct {
			GUID  string `json:"guid"`
			Asset struct {
				Stills json.RawMessage `json:"stills"`
			} `json:"asset"`
		}
		if json.Unmarshal(sc.Bytes(), &line) == nil && line.GUID == "R1" {
			json.Unmarshal(line.Asset.Stills, &stills)
		}
	}
	if len(stills) != 1 || stills[0].URL != "/assets/R1/r/400.webp" || stills[0].W != 400 || stills[0].Mime != "image/webp" {
		t.Fatalf("stills %+v", stills)
	}
	if rec := get(e, stills[0].URL); rec.Code != http.StatusOK || rec.Body.String() != "webp" {
		t.Errorf("the rendition: %d %q", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/assets/R1/r/1600.webp", "/assets/OTHER/r/400.webp", "/assets/R1/r/..%2F..%2Fx"} {
		if code := get(e, path).Code; code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, code)
		}
	}
}
