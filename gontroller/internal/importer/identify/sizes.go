package identify

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"

	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
	_ "golang.org/x/image/webp"
)

// SizesStore: what sizes writes — the files' pixels and codecs — and, when what is
// read of the files changes, the version it was read with and every item sent again
type SizesStore interface {
	UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error)
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
	GetAllGUIDs() ([]api.GUID, error)
	MarkRework(guids []api.GUID) (int64, error)
}

// What setSizes stores of the files changes: a new readVersion sends every item
// through the cheap stage once more (its files read again). Not its fingerprint:
// that would make render redo everything, videos too.
const readVersion = "2" // 2: the files' own Orientation (dto.FileDto.StaleOrientation)

const readVersionKey = "read_version"

// setSizes: the original's size comes from its metadata (the source's first: the
// Photos DB size is oriented); images from their header — no decoding; a remote
// file's from its source (its grouper set them). Each file's own Orientation with
// it; a main file whose tag is stale (dto.FileDto.StaleOrientation) gives the
// asset "as its pixels are" — the size perceptor then turns nothing.
func setSizes(it *draft) {
	for i, f := range it.Files {
		if f.Role == dto.RoleMeta || f.Remote() {
			continue
		}
		if it.Exif[i] != nil {
			f.Codec = string(it.Exif[i]["CompressorID"])
			f.Orientation, _ = strconv.Atoi(string(it.Exif[i]["Orientation"]))
		}
		// The metadata describes the original: a derivative standing in as the main
		// file (a cloud-only asset) has its own size
		if i == 0 && f.Role == dto.RoleOriginal {
			f.Orientation, _ = strconv.Atoi(mainTag(it, "Orientation"))
			w, _ := strconv.Atoi(mainTag(it, "ImageWidth"))
			h, _ := strconv.Atoi(mainTag(it, "ImageHeight"))
			if w > 0 && h > 0 {
				f.Width, f.Height = w, h
				continue
			}
		}
		if w, h, ok := headerSize(f.Path); ok {
			f.Width, f.Height = w, h
		}
	}
	if len(it.Files) > 0 && it.Files[0].StaleOrientation() {
		it.Merged["Orientation"] = []byte("1")
	}
}

// readFilesAgain runs at start: a new readVersion marks every item for another pass
func readFilesAgain(db SizesStore, logger *l.Logger) error {
	stored, err := db.GetMeta(readVersionKey)
	if err != nil || stored == readVersion {
		return err
	}
	guids, err := db.GetAllGUIDs()
	if err != nil {
		return err
	}
	n, err := db.MarkRework(guids)
	if err != nil {
		return err
	}
	logger.Info("What is read of the files changed: every item is identified once more",
		l.String("from", stored), l.String("to", readVersion), l.Int("items", int(n)))
	return db.SetMeta(readVersionKey, readVersion)
}

// mainTag: a tag of the main file — the source's metadata first, then its EXIF
func mainTag(it *draft, tag string) string {
	if v, ok := it.Meta[tag]; ok {
		return string(v)
	}
	if it.Exif[0] != nil {
		return string(it.Exif[0][tag])
	}
	return ""
}

// headerSize reads an image's size from its header (JPEG, PNG, GIF, WebP)
func headerSize(path string) (int, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}
