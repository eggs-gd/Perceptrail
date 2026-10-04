package route

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"

	"github.com/labstack/echo/v4"
)

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "routes-test")
	if err := model.Configure(model.DBConfig{Driver: model.DriverSQLite, Name: filepath.Join(dir, "t.db")}); err != nil {
		panic(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	itemsProxy = model.NewProxy(logger)
	filesProxy = model.NewProxy(logger)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestClientAssetByRoles(t *testing.T) {
	item := &dto.ItemDto{Guid: "G", PreviewPath: "/cache/G/embedded.jpg", PreviewMime: "image/jpeg"}
	file := func(id uint, role, mime string, w int) *dto.FileDto {
		f := &dto.FileDto{ID: id, Role: role, Width: w, LinkedTo: "G"}
		f.MimeType = mime
		return f
	}
	a := toClientAsset(item, []*dto.FileDto{
		file(1, dto.RoleOriginal, "image/heic", 4032),
		file(2, dto.RoleStill, "image/jpeg", 2048),
		file(3, dto.RoleStill, "image/jpeg", 360),
		file(4, dto.RoleEdit, "image/jpeg", 4032),
		file(5, dto.RoleMotion, "video/quicktime", 1080),
		file(6, dto.RoleMeta, "application/rdf+xml", 0),
	})
	if a.Original == nil || a.Original.URL != "/assets/G/1" || a.Original.Mime != "image/heic" {
		t.Errorf("original %+v", a.Original)
	}
	// Smallest first; the extracted embedded preview (unknown size here) is a still too
	if len(a.Stills) != 3 || a.Stills[0].URL != "/assets/G/embedded" || a.Stills[1].W != 360 || a.Stills[2].W != 2048 {
		t.Errorf("stills %+v", a.Stills)
	}
	if len(a.Edit) != 1 || len(a.Motion) != 1 || len(a.Frames) != 0 {
		t.Errorf("edit %+v motion %+v frames %+v", a.Edit, a.Motion, a.Frames)
	}
}

// The kind: the source's word first (an Apple Live Photo's original is its video),
// else by the roles
func TestClientAssetKind(t *testing.T) {
	file := func(role, mime string) *dto.FileDto {
		f := &dto.FileDto{Role: role}
		f.MimeType = mime
		return f
	}
	for _, c := range []struct {
		name, stored string
		files        []*dto.FileDto
		want         string
	}{
		{"photo", "", []*dto.FileDto{file(dto.RoleOriginal, "image/heic")}, dto.KindPhoto},
		{"generic live", "", []*dto.FileDto{file(dto.RoleOriginal, "image/heic"), file(dto.RoleMotion, "video/quicktime")}, dto.KindLive},
		{"video", "", []*dto.FileDto{file(dto.RoleOriginal, "video/mp4"), file(dto.RoleStill, "image/jpeg")}, dto.KindVideo},
		{"apple live", dto.KindLive, []*dto.FileDto{file(dto.RoleOriginal, "video/quicktime"), file(dto.RoleStill, "image/heic")}, dto.KindLive},
	} {
		if got := toClientAsset(&dto.ItemDto{Guid: "G", Kind: c.stored}, c.files).Kind; got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A file is served only for its own asset
func TestAssetFileOnlyOfItsAsset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.jpg")
	os.WriteFile(path, []byte("jpeg"), 0o644)
	f, err := filesProxy.CreateFile(dto.ItemEntry{Path: path, Name: "a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	f.LinkToItem("MINE")
	if _, err := filesProxy.UpdateFile(f); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	get := func(guid, name string) int {
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
		c.SetParamNames("item", "file")
		c.SetParamValues(guid, name)
		if err := getAssetFile(c); err != nil {
			if he, ok := err.(*echo.HTTPError); ok {
				return he.Code
			}
			return 500
		}
		return rec.Code
	}
	id := func() string { return itoa(f.ID) }
	if code := get("MINE", id()); code != http.StatusOK {
		t.Errorf("own file: %d", code)
	}
	if code := get("OTHER", id()); code != http.StatusNotFound {
		t.Errorf("another asset's file: %d, want 404", code)
	}
	if code := get("MINE", "../../etc/passwd"); code != http.StatusNotFound {
		t.Errorf("a path: %d, want 404", code)
	}
}

func itoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}
