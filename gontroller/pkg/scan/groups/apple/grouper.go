// Package apple groups an Apple Photos library (*.photoslibrary) by its database:
// the first file of a library loads the assets from Photos.sqlite and forms the
// groups in memory — the files of every asset that exist on disk now — then the
// walker's files fill them; a group goes out when its last file has arrived. One
// group is one asset: the key is the asset UUID (the item's GUID), the main file
// is the source (the original; its Live Photo video before it), the rest is
// linked to it. We only read the library, never write it.
//
// File layout (see findings "Apple Photos library: spike"): only the original's
// path is in the DB; renders and derivatives follow a naming layout by UUID.
package apple

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/api"
	"perceptrail/gontroller/pkg/scan/flow"

	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"

	_ "github.com/mattn/go-sqlite3"
)

type Grouper struct {
	logger *l.Logger
	libs   map[string]*library // by bundle path, loaded once per walk
}

func NewGrouper(chin <-chan flow.FileEvent, chout chan<- flow.FileGroup, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, NewDecorator(logger))
}

// NewDecorator: the grouper itself, for tests and custom wiring
func NewDecorator(logger *l.Logger) *Grouper {
	return &Grouper{logger: logger, libs: map[string]*library{}}
}

func (g *Grouper) Decorate(ev flow.FileEvent) (flow.FileGroup, error) {
	if ev.Done != nil {
		// Groups that did not complete (a file vanished during the walk) wait for the
		// next walk; their files must not count as gone
		var held []string
		for _, lib := range g.libs {
			held = append(held, lib.pending()...)
		}
		g.libs = map[string]*library{}
		return flow.FileGroup{Done: ev.Done, Held: held}, nil
	}

	root := BundleRoot(ev.Entry.Path)
	if root == "" {
		return flow.FileGroup{}, fmt.Errorf("apple grouper: %s is not in a Photos library", ev.Entry.Path)
	}
	lib, ok := g.libs[root]
	if !ok {
		var err error
		if lib, err = loadLibrary(root); err != nil {
			g.logger.Error("Photos library not loaded", l.String("library", root), l.Error(err))
			lib = &library{byPath: map[string]*asset{}} // nothing groups this walk
		} else {
			g.logger.Info("Photos library loaded", l.String("library", root), l.Int("assets", len(lib.assets)))
		}
		g.libs[root] = lib
	}

	a, ok := lib.byPath[ev.Entry.Path]
	if !ok || a.sent {
		return flow.FileGroup{}, chain.ErrSkippedItem // not an asset file (caches, DB, …)
	}
	a.arrived[ev.Entry.Path] = &dto.FileDto{ItemEntry: ev.Entry}
	if len(a.arrived) < len(a.files) {
		return flow.FileGroup{}, chain.ErrSkippedItem // not complete yet
	}
	a.sent = true
	return a.group(), nil
}

func (g *Grouper) Stop() {}

// BundleRoot: the *.photoslibrary directory path contains, "" if none
func BundleRoot(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, p := range parts {
		if strings.HasSuffix(strings.ToLower(p), ".photoslibrary") {
			return filepath.FromSlash(strings.Join(parts[:i+1], "/"))
		}
	}
	return ""
}

type library struct {
	assets []*asset
	byPath map[string]*asset
}

// pending: the files of groups that did not go out
func (lib *library) pending() []string {
	var held []string
	for _, a := range lib.assets {
		if a.sent || len(a.arrived) == 0 {
			continue
		}
		for _, f := range a.files {
			held = append(held, f.path)
		}
	}
	return held
}

type asset struct {
	uuid     string
	meta     api.RawExif // the DB's metadata (wins over the files' EXIF)
	metaHash string
	kind     string      // dto.Kind*
	files    []candidate // on disk at load time, in group order: the main file first
	show     []string    // what to show first, best first
	arrived  map[string]*dto.FileDto
	sent     bool
}

type candidate struct {
	path string
	role role
}

func (a *asset) group() flow.FileGroup {
	g := flow.FileGroup{Key: a.uuid, Meta: a.meta, MetaHash: a.metaHash, Kind: a.kind}
	for _, c := range a.files {
		f := a.arrived[c.path]
		f.Role = c.role.fileRole()
		if c.role == roleOriginal && a.files[0].role == roleLiveVideo {
			f.Role = dto.RoleStill // a Live Photo's photo: its video is the source
		}
		g.Files = append(g.Files, f)
	}
	for _, p := range a.show {
		g.Show = append(g.Show, a.arrived[p])
	}
	return g
}

// role: what a file is to its asset. The order is the group order (the main file
// is the first that exists); showRank orders what to show.
type role int

const (
	roleLiveVideo role = iota // the video of a Live Photo: the source when present
	roleOriginal
	roleRender      // the user's edit, full size
	roleRenderHEIC  // the same as HEIC (not viewable outside Safari)
	roleEditPreview // ~2000 px of the edit
	roleLarge       // ~2000–2600 px of the original
	roleLarge2      // ~2000 px of the original
	roleMedium      // ~1000 px
	roleMedium2     // ~1000 px
	roleThumb       // the small thumbnail (~360×640)
	roleVideoPoster // .THM (32×32)
	roleFrame       // cvt/…/_cvt_tNNNN.jpeg: frames of a video (a flip-book)
)

// fileRole: what the file is to the asset, for the client
func (r role) fileRole() string {
	switch r {
	case roleLiveVideo, roleOriginal:
		return dto.RoleOriginal
	case roleRender, roleRenderHEIC, roleEditPreview:
		return dto.RoleEdit
	case roleFrame:
		return dto.RoleFrames
	default:
		return dto.RoleStill
	}
}

// showRank: the edit first (it is what the user sees in Photos), then the original,
// then the biggest derivative; the cheap stage takes the first the browser shows
var showRank = []role{roleRender, roleEditPreview, roleRenderHEIC, roleOriginal, roleLarge, roleLarge2, roleMedium, roleMedium2, roleThumb, roleVideoPoster}

// candidates: every file an asset may have, per the naming layout
func candidates(root, uuid, dir, filename string) []candidate {
	x := strings.ToUpper(uuid[:1])
	deriv := filepath.Join(root, "resources", "derivatives", x)
	render := filepath.Join(root, "resources", "renders", x)
	return []candidate{
		{filepath.Join(root, "originals", x, uuid+"_3.mov"), roleLiveVideo},
		{filepath.Join(root, "originals", dir, filename), roleOriginal},
		{filepath.Join(render, uuid+"_1_201_a.jpeg"), roleRender},
		{filepath.Join(render, uuid+"_1_201_a.heic"), roleRenderHEIC},
		{filepath.Join(deriv, uuid+"_1_102_a.jpeg"), roleEditPreview},
		{filepath.Join(deriv, uuid+"_1_101_o.jpeg"), roleLarge},
		{filepath.Join(deriv, uuid+"_1_102_o.jpeg"), roleLarge2},
		{filepath.Join(deriv, uuid+"_1_105_c.jpeg"), roleMedium},
		{filepath.Join(deriv, uuid+"_1_106_c.jpeg"), roleMedium2},
		{filepath.Join(root, "resources", "derivatives", "masters", x, uuid+"_4_5005_c.jpeg"), roleThumb},
		{filepath.Join(deriv, uuid+".THM"), roleVideoPoster},
	}
}

// frames: the frames Photos keeps for a video (resources/derivatives/cvt/<X>/<UUID>/),
// in order
func frames(root, uuid string) []candidate {
	dir := filepath.Join(root, "resources", "derivatives", "cvt", strings.ToUpper(uuid[:1]), uuid)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []candidate
	for _, e := range entries { // ReadDir sorts by name: _t0000, _t0001, …
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jpeg") {
			out = append(out, candidate{filepath.Join(dir, e.Name()), roleFrame})
		}
	}
	return out
}

// loadLibrary reads the assets from a copy of the DB (Photos may have it open,
// the library may be a share or a copy) and keeps the files that exist
func loadLibrary(root string) (*library, error) {
	rows, err := readAssets(root)
	if err != nil {
		return nil, err
	}
	lib := &library{byPath: map[string]*asset{}}
	for _, r := range rows {
		if r.trashed || r.hidden || len(r.uuid) < 2 {
			continue
		}
		meta := r.meta.record()
		a := &asset{uuid: r.uuid, arrived: map[string]*dto.FileDto{}, meta: meta, kind: r.kind(),
			metaHash: hashRecord(meta, r.kind())}
		byRole := map[role]string{}
		for _, c := range candidates(root, r.uuid, r.dir, r.filename) {
			if info, err := os.Stat(c.path); err == nil && !info.IsDir() {
				a.files = append(a.files, c)
				byRole[c.role] = c.path
			}
		}
		a.files = append(a.files, frames(root, r.uuid)...)
		if len(a.files) == 0 {
			continue // nothing local: not even a thumbnail
		}
		for _, r := range showRank {
			if p, ok := byRole[r]; ok {
				a.show = append(a.show, p)
			}
		}
		lib.assets = append(lib.assets, a)
		for _, c := range a.files {
			lib.byPath[c.path] = a
		}
	}
	return lib, nil
}

type assetRow struct {
	uuid, dir, filename string
	trashed, hidden     bool
	meta                assetMeta
	// ZKIND: 0 photo, 1 video; ZPLAYBACKSTYLE: 3 a Live Photo (live on). A Live
	// Photo with live switched off is a still in Photos (ZKINDSUBTYPE 2, style 1).
	zkind, playback int
}

func (r assetRow) kind() string {
	switch {
	case r.zkind == 1:
		return dto.KindVideo
	case r.playback == 3:
		return dto.KindLive
	}
	return dto.KindPhoto
}

func readAssets(root string) ([]assetRow, error) {
	tmp, err := os.MkdirTemp("", "photos-db-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := filepath.Join(root, "database", "Photos.sqlite"+suffix)
		if err := copyFile(src, filepath.Join(tmp, "Photos.sqlite"+suffix)); err != nil {
			if suffix == "" || !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}

	db, err := sql.Open("sqlite3", "file:"+filepath.Join(tmp, "Photos.sqlite")+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	// CAST: ZDATECREATED is declared TIMESTAMP; a whole-second value is stored as an
	// integer, which the driver would turn into a time.Time
	rs, err := db.Query(`SELECT a.ZUUID, ifnull(a.ZDIRECTORY,''), ifnull(a.ZFILENAME,''),
		ifnull(a.ZTRASHEDSTATE,0), ifnull(a.ZHIDDEN,0), ifnull(a.ZKIND,0), ifnull(a.ZPLAYBACKSTYLE,0),
		CAST(a.ZDATECREATED AS REAL), a.ZWIDTH, a.ZHEIGHT, a.ZLATITUDE, a.ZLONGITUDE, x.ZTIMEZONEOFFSET, a.ZDURATION
		FROM ZASSET a LEFT JOIN ZADDITIONALASSETATTRIBUTES x ON x.ZASSET = a.Z_PK`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []assetRow
	for rs.Next() {
		var r assetRow
		var trashed, hidden int
		m := &r.meta
		if err := rs.Scan(&r.uuid, &r.dir, &r.filename, &trashed, &hidden, &r.zkind, &r.playback,
			&m.created, &m.width, &m.height, &m.lat, &m.lon, &m.tzOffset, &m.duration); err != nil {
			return nil, err
		}
		r.trashed, r.hidden = trashed != 0, hidden != 0
		out = append(out, r)
	}
	return out, rs.Err()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
