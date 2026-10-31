package identify

import (
	"testing"

	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
	"github.com/eggs-gd/perceplib/api"
)

// noTool: an Exiftool a remote group must never reach
type noTool struct{ t *testing.T }

func (n noTool) Read(paths, tags []string) ([]api.RawExif, error) {
	n.t.Errorf("exiftool read %v", paths)
	return nil, nil
}
func (n noTool) Extract(tag, src, dst string) error {
	n.t.Errorf("exiftool extracted %s of %s", tag, src)
	return nil
}
func (n noTool) Close() {}

// A remote asset (Immich): nothing read from its files — its metadata and its
// fingerprint are its source's, its show is the source's still
func TestReadRemote(t *testing.T) {
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	file := func(name, mime, role string) *dto.FileDto {
		return &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "immich://a1/" + name, Name: name, MimeType: mime}, Role: role}
	}
	original, still := file("IMG_1.heic", "image/heic", dto.RoleOriginal), file("preview.jpg", "image/jpeg", dto.RoleStill)
	asset := dto.Asset{
		Key: "a1", Fingerprint: "sum", Files: []*dto.FileDto{original, still}, Show: []*dto.FileDto{still},
		Meta: api.RawExif{"Make": []byte("Apple"), "ImageWidth": []byte("4032")},
	}
	d, err := newReader(noTool{t}, []string{"Make"}, logger).Decorate(asset)
	if err != nil {
		t.Fatal(err)
	}
	if d.Hash != "sum" || string(d.Merged["Make"]) != "Apple" || d.Kinds[0] != kindImage || d.Exif[1] != nil {
		t.Errorf("hash %q, merged %v, kinds %v, exif %v", d.Hash, d.Merged, d.Kinds, d.Exif)
	}
	if asset.Meta["MIMEType"] != nil {
		t.Error("the source's metadata changed")
	}
	d.Item = &dto.ItemDto{GUID: "a1"}
	if path, _ := preview(d, noTool{t}, "/cache", logger); path != still.Path {
		t.Errorf("preview %q", path)
	}
	d.Show, d.Files, d.Kinds, d.Exif = nil, d.Files[:1], d.Kinds[:1], d.Exif[:1] // a HEIC alone: waits, nothing extracted
	if path, _ := preview(d, noTool{t}, "/cache", logger); path != "" {
		t.Errorf("preview %q", path)
	}
}
