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
			// Nothing groups this walk, and its files are held: a DB that cannot be read
			// says nothing about which assets are gone
			g.logger.Error("Photos library not loaded: its files are held until it loads", l.String("library", root), l.Error(err))
			lib = &library{byPath: map[string]*asset{}, failed: true}
		} else {
			g.logger.Info("Photos library loaded", l.String("library", root), l.Int("assets", len(lib.assets)))
		}
		g.libs[root] = lib
	}

	if lib.failed {
		lib.held = append(lib.held, ev.Entry.Path)
		return flow.FileGroup{}, chain.ErrSkippedItem
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

// HasLibrary: root is a Photos library or holds one at its top (where Photos keeps
// it: ~/Pictures) — then Photos is worth asking for access
func HasLibrary(root string) bool {
	if strings.HasSuffix(strings.ToLower(root), ".photoslibrary") {
		return true
	}
	m, _ := filepath.Glob(filepath.Join(root, "*.photoslibrary"))
	return len(m) > 0
}

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
	failed bool     // the DB did not load
	held   []string // a failed library's files, as walked
}

// pending: the files of groups that did not go out; all of a failed library's
func (lib *library) pending() []string {
	held := lib.held
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
	// Video renditions Photos downloads on request (PhotoKit): after the stills, so
	// the main file does not change when one appears. _a: of the user's edit (what
	// Photos shows; seen for a Live Photo's motion, assumed for videos), _o: of the
	// original.
	roleLiveMotionEdit  // _2_101_a.mov
	roleVideoHEVCEdit   // _2_201_a.mov
	roleVideoMediumEdit // _2_3_a.mp4
	roleVideoSmallEdit  // _2_4_a.mp4
	roleVideoHEVC       // _2_201_o.mov: 720p HEVC (an iPhone video's medium)
	roleVideoMedium     // _2_3_o.mp4: 720p H.264 (another video's medium)
	roleVideoSmall      // _2_4_o.mp4: 360p H.264 (fast)
	roleLiveMotion      // _2_101_o.mov: a Live Photo's motion, H.264
	roleFrame           // cvt/…/_cvt_tNNNN.jpeg: frames of a video (a flip-book)
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
	case roleLiveMotionEdit, roleVideoHEVCEdit, roleVideoMediumEdit, roleVideoSmallEdit,
		roleVideoHEVC, roleVideoMedium, roleVideoSmall, roleLiveMotion:
		return dto.RoleMotion
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
		{filepath.Join(deriv, uuid+"_2_101_a.mov"), roleLiveMotionEdit},
		{filepath.Join(deriv, uuid+"_2_201_a.mov"), roleVideoHEVCEdit},
		{filepath.Join(deriv, uuid+"_2_3_a.mp4"), roleVideoMediumEdit},
		{filepath.Join(deriv, uuid+"_2_4_a.mp4"), roleVideoSmallEdit},
		{filepath.Join(deriv, uuid+"_2_201_o.mov"), roleVideoHEVC},
		{filepath.Join(deriv, uuid+"_2_3_o.mp4"), roleVideoMedium},
		{filepath.Join(deriv, uuid+"_2_4_o.mp4"), roleVideoSmall},
		{filepath.Join(deriv, uuid+"_2_101_o.mov"), roleLiveMotion},
	}
}

// Want: what the client asks for on demand (the viewer, a hover)
type Want int

const (
	WantImage      Want = iota // the viewer's image: the edit, else ~2048 px
	WantVideo                  // the viewer's video: 720p, HEVC allowed
	WantVideoH264              // the same for a browser that plays no HEVC
	WantVideoHover             // a video on hover: the smallest H.264
	WantLiveMotion             // a Live Photo's motion
)

// wanted: the renditions that answer a want, best first. Not the original: its
// path is in the DB, not in the naming layout (the caller has it).
var wanted = map[Want][]role{
	WantImage:      {roleRender, roleEditPreview, roleLarge, roleLarge2},
	WantVideo:      {roleVideoHEVCEdit, roleVideoMediumEdit, roleVideoHEVC, roleVideoMedium},
	WantVideoH264:  {roleVideoMediumEdit, roleVideoSmallEdit, roleVideoMedium, roleVideoSmall},
	WantVideoHover: {roleVideoSmallEdit, roleVideoMediumEdit, roleVideoSmall, roleVideoMedium},
	WantLiveMotion: {roleLiveMotionEdit, roleLiveMotion, roleLiveVideo},
}

// Local: the best file of the asset in the library for want, "" if none is local
// yet — then Photos is asked for it (PhotoKit) and Local looks again
func Local(root, uuid string, want Want) string {
	byRole := map[role]string{}
	for _, c := range candidates(root, uuid, "", "") {
		byRole[c.role] = c.path
	}
	for _, r := range wanted[want] {
		if p, ok := byRole[r]; ok {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p
			}
		}
	}
	return ""
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
