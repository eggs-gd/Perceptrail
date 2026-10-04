package apple

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

const (
	edited    = "A1111111-0000-0000-0000-000000000001" // original HEIC + edit + derivatives
	cloudOnly = "B2222222-0000-0000-0000-000000000002" // no original: derivatives only
	live      = "C3333333-0000-0000-0000-000000000003" // Live Photo: photo + video
	trashed   = "D4444444-0000-0000-0000-000000000004"
	vanishing = "E5555555-0000-0000-0000-000000000005" // a file disappears during the walk
)

type fixtureAsset struct {
	uuid, dir, filename string
	trashed             bool
	zkind, playback     int
	duration            float64
	files               []string // relative to the bundle
}

// makeLibrary: a minimal Photos library — ZASSET with the columns we read, plus
// the files. Returns the bundle path.
func makeLibrary(t *testing.T, root string, assets []fixtureAsset) string {
	t.Helper()
	bundle := filepath.Join(root, "Photos Library.photoslibrary")
	if err := os.MkdirAll(filepath.Join(bundle, "database"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(bundle, "database", "Photos.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ZASSET (Z_PK INTEGER PRIMARY KEY, ZUUID VARCHAR,
		ZDIRECTORY VARCHAR, ZFILENAME VARCHAR, ZTRASHEDSTATE INTEGER, ZHIDDEN INTEGER,
		ZKIND INTEGER, ZPLAYBACKSTYLE INTEGER, ZDURATION FLOAT, ZDATECREATED TIMESTAMP, ZWIDTH INTEGER, ZHEIGHT INTEGER, ZLATITUDE FLOAT, ZLONGITUDE FLOAT);
		CREATE TABLE ZADDITIONALASSETATTRIBUTES (Z_PK INTEGER PRIMARY KEY, ZASSET INTEGER,
		ZTIMEZONEOFFSET INTEGER)`); err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		tr := 0
		if a.trashed {
			tr = 1
		}
		res, err := db.Exec(`INSERT INTO ZASSET (ZUUID, ZDIRECTORY, ZFILENAME, ZTRASHEDSTATE, ZHIDDEN, ZKIND, ZPLAYBACKSTYLE, ZDURATION,
			ZDATECREATED, ZWIDTH, ZHEIGHT, ZLATITUDE, ZLONGITUDE) VALUES (?,?,?,?,0,?,?,?, 758992569.5, 3024, 4032, 50.4293, -30.5381)`,
			a.uuid, a.dir, a.filename, tr, a.zkind, a.playback, a.duration)
		if err != nil {
			t.Fatal(err)
		}
		pk, _ := res.LastInsertId()
		if _, err := db.Exec(`INSERT INTO ZADDITIONALASSETATTRIBUTES (ZASSET, ZTIMEZONEOFFSET) VALUES (?, 7200)`, pk); err != nil {
			t.Fatal(err)
		}
		for _, f := range a.files {
			p := filepath.Join(bundle, f)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Photos' own files: not assets
	for _, f := range []string{"resources/caches/x.data", "database/search/psi.sqlite"} {
		p := filepath.Join(bundle, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return bundle
}

func fixture() []fixtureAsset {
	return []fixtureAsset{
		{uuid: edited, dir: "A", filename: edited + ".heic", files: []string{
			"originals/A/" + edited + ".heic",
			"resources/renders/A/" + edited + "_1_201_a.jpeg",
			"resources/derivatives/A/" + edited + "_1_102_o.jpeg",
			"resources/derivatives/masters/A/" + edited + "_4_5005_c.jpeg",
		}},
		{uuid: cloudOnly, dir: "B", filename: cloudOnly + ".jpeg", files: []string{
			"resources/derivatives/B/" + cloudOnly + "_1_105_c.jpeg",
			"resources/derivatives/masters/B/" + cloudOnly + "_4_5005_c.jpeg",
		}},
		{uuid: live, dir: "C", filename: live + ".heic", playback: 3, duration: 2.5, files: []string{
			"originals/C/" + live + ".heic",
			"originals/C/" + live + "_3.mov",
			"resources/derivatives/C/" + live + "_1_102_o.jpeg",
			"resources/derivatives/cvt/C/" + live + "/" + live + "_cvt_t0000.jpeg",
			"resources/derivatives/cvt/C/" + live + "/" + live + "_cvt_t0001.jpeg",
		}},
		{uuid: trashed, dir: "D", filename: trashed + ".jpeg", trashed: true, files: []string{
			"originals/D/" + trashed + ".jpeg",
		}},
		{uuid: vanishing, dir: "E", filename: vanishing + ".jpeg", files: []string{
			"originals/E/" + vanishing + ".jpeg",
			"resources/derivatives/masters/E/" + vanishing + "_4_5005_c.jpeg",
		}},
	}
}

// walk feeds every file under root in the walker's order (sorted paths), then the
// walk's flush: the groups by key (the files sent as gone under "")
func walk(t *testing.T, g *Grouper, root string, before func(path string)) map[string]dto.Asset {
	t.Helper()
	var paths []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	groups := map[string]dto.Asset{}
	for _, p := range paths {
		if before != nil {
			before(p)
		}
		if _, err := os.Stat(p); err != nil {
			continue // vanished: the walker would not see it
		}
		out, err := g.Decorate(&dto.FileDto{ItemEntry: dto.ItemEntry{Path: p, Name: filepath.Base(p)}})
		if errors.Is(err, chain.ErrSkippedItem) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Files) == 1 && out.Files[0].Gone {
			gone := groups[""]
			gone.Files = append(gone.Files, out.Files...)
			groups[""] = gone
			continue
		}
		if _, dup := groups[out.Key]; dup {
			t.Fatalf("asset %s sent twice", out.Key)
		}
		groups[out.Key] = out
	}
	if held, err := g.Flush(); err != nil || len(held) != 0 {
		t.Fatalf("the flush gave %v, %v", held, err)
	}
	return groups
}

func names(files []*dto.FileDto) []string {
	var out []string
	for _, f := range files {
		out = append(out, filepath.Base(f.Path))
	}
	return out
}

func TestGrouper(t *testing.T) {
	root := t.TempDir()
	bundle := makeLibrary(t, root, fixture())
	g := newGrouper(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))

	vanished := filepath.Join(bundle, "resources/derivatives/masters/E/"+vanishing+"_4_5005_c.jpeg")
	groups := walk(t, g, root, func(p string) {
		if filepath.Base(p) == filepath.Base(vanished) {
			os.Remove(vanished) // Photos purged it after the library was loaded
		}
	})
	gone := groups[""]
	delete(groups, "")

	if len(groups) != 3 {
		t.Fatalf("groups %v, want edited, cloud-only, live", groups)
	}
	ed := groups[edited]
	if got := names(ed.Files); got[0] != edited+".heic" || len(got) != 4 {
		t.Errorf("edited: files %v, want the original first", got)
	}
	if got := names(ed.Show); got[0] != edited+"_1_201_a.jpeg" || got[1] != edited+".heic" {
		t.Errorf("edited: show %v, want the edit, then the original", got)
	}
	if got := names(groups[cloudOnly].Files); got[0] != cloudOnly+"_1_105_c.jpeg" {
		t.Errorf("cloud-only: main %v, want the biggest derivative", got)
	}
	if got := names(groups[live].Files); got[0] != live+"_3.mov" || got[1] != live+".heic" {
		t.Errorf("live: files %v, want the video (the source), then the photo", got)
	}
	if _, ok := groups[trashed]; ok {
		t.Error("a trashed asset was sent")
	}
	if len(gone.Files) == 0 || !strings.Contains(gone.Files[0].Path, trashed) {
		t.Errorf("gone %v, want the trashed asset's files", names(gone.Files))
	}
	if _, ok := groups[vanishing]; ok {
		t.Error("an asset whose file vanished during the walk was sent")
	}

	// The next walk loads the library again: the vanished file is not expected
	groups = walk(t, g, root, nil)
	if _, ok := groups[vanishing]; !ok {
		t.Errorf("next walk: %v", groups[vanishing])
	}
}

// A file the walk says is gone passes through as it is, an asset of its own
func TestGrouperGone(t *testing.T) {
	g := newGrouper(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	gone := &dto.FileDto{ItemEntry: dto.ItemEntry{Path: "/lib/x.photoslibrary/originals/A/A.heic"}, Gone: true}
	out, err := g.Decorate(gone)
	if err != nil || len(out.Files) != 1 || out.Files[0] != gone {
		t.Errorf("got %+v, %v", out, err)
	}
}

// A DB that cannot be read groups nothing (the walk stamped its files: none is
// taken for gone)
func TestGrouperUnreadableLibrary(t *testing.T) {
	root := t.TempDir()
	bundle := makeLibrary(t, root, fixture())
	if err := os.WriteFile(filepath.Join(bundle, "database", "Photos.sqlite"), []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := newGrouper(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	if groups := walk(t, g, root, nil); len(groups) != 0 {
		t.Errorf("groups %v, want none", groups)
	}
}

func TestBundleRoot(t *testing.T) {
	if got := BundleRoot("/Pictures/Photos Library.photoslibrary/originals/A/x.heic"); got != "/Pictures/Photos Library.photoslibrary" {
		t.Errorf("got %q", got)
	}
	if got := BundleRoot("/Pictures/a.jpg"); got != "" {
		t.Errorf("got %q", got)
	}
}

// The DB's metadata as exiftool would print it: the local time + its offset, the
// oriented size (the file's Orientation must not turn it again), the place
func TestMetaRecord(t *testing.T) {
	root := t.TempDir()
	makeLibrary(t, root, fixture())
	g := newGrouper(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	groups := walk(t, g, root, nil)
	m := groups[edited].Meta
	want := map[string]string{
		// 758992569.5 s after 2001-01-01 UTC = 2025-01-19 15:16:09.5 UTC, +02:00
		"DateTimeOriginal":   "2025:01:19 17:16:09",
		"OffsetTimeOriginal": "+02:00",
		"SubSecTimeOriginal": "500",
		"ImageWidth":         "3024",
		"ImageHeight":        "4032",
		"Orientation":        "1",
		"GPSLatitude":        "50.4293",
		"GPSLongitude":       "-30.5381",
	}
	for k, v := range want {
		if got := string(m[k]); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if groups[edited].MetaHash == "" {
		t.Error("no meta hash")
	}
	if k := groups[edited].Kind; k != dto.KindPhoto {
		t.Errorf("edited: kind %q, want photo", k)
	}
	if d := string(groups[live].Meta["Duration"]); d != "2.5" {
		t.Errorf("live: Duration %q, want 2.5 (seconds, as exiftool -n)", d)
	}
	if _, ok := groups[edited].Meta["Duration"]; ok {
		t.Error("a photo got a Duration")
	}
	if k := groups[live].Kind; k != dto.KindLive {
		t.Errorf("live: kind %q, want live", k)
	}
}

// Roles for the client: the source, the edit, stills, frames of a video
func TestGrouperRoles(t *testing.T) {
	root := t.TempDir()
	makeLibrary(t, root, fixture())
	g := newGrouper(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	groups := walk(t, g, root, nil)

	roles := func(uuid string) map[string]string {
		out := map[string]string{}
		for _, f := range groups[uuid].Files {
			out[filepath.Base(f.Path)] = f.Role
		}
		return out
	}
	for name, want := range map[string]string{
		edited + ".heic":          dto.RoleOriginal,
		edited + "_1_201_a.jpeg":  dto.RoleEdit,
		edited + "_1_102_o.jpeg":  dto.RoleStill,
		edited + "_4_5005_c.jpeg": dto.RoleStill,
	} {
		if got := roles(edited)[name]; got != want {
			t.Errorf("edited %s: %q, want %q", name, got, want)
		}
	}
	for name, want := range map[string]string{
		live + "_3.mov":          dto.RoleOriginal, // the video is the source
		live + ".heic":           dto.RoleStill,
		live + "_cvt_t0000.jpeg": dto.RoleFrames,
		live + "_cvt_t0001.jpeg": dto.RoleFrames,
	} {
		if got := roles(live)[name]; got != want {
			t.Errorf("live %s: %q, want %q", name, got, want)
		}
	}
}

// The renditions Photos downloads on request: videos are the asset's motion, the
// main file stays what it was (they come after the stills); Local finds the best
// one for a want
func TestGrouperVideoRenditions(t *testing.T) {
	const video = "F6666666-0000-0000-0000-000000000006" // cloud-only, two renditions fetched
	const livePhoto = "G7777777-0000-0000-0000-000000000007"
	root := t.TempDir()
	bundle := makeLibrary(t, root, []fixtureAsset{
		{uuid: video, dir: "F", filename: video + ".mov", zkind: 1, duration: 8, files: []string{
			"resources/derivatives/masters/F/" + video + "_4_5005_c.jpeg",
			"resources/derivatives/F/" + video + "_2_201_o.mov",
			"resources/derivatives/F/" + video + "_2_4_o.mp4",
		}},
		{uuid: livePhoto, dir: "G", filename: livePhoto + ".heic", playback: 3, files: []string{
			"resources/derivatives/G/" + livePhoto + "_1_102_o.jpeg",
			"resources/derivatives/G/" + livePhoto + "_2_101_a.mov",
			"resources/derivatives/G/" + livePhoto + "_2_101_o.mov",
		}},
	})
	g := newGrouper(l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{}))
	groups := walk(t, g, root, nil)

	for uuid, want := range map[string][]string{
		video:     {video + "_4_5005_c.jpeg", dto.RoleStill, video + "_2_201_o.mov", dto.RoleMotion, video + "_2_4_o.mp4", dto.RoleMotion},
		livePhoto: {livePhoto + "_1_102_o.jpeg", dto.RoleStill, livePhoto + "_2_101_a.mov", dto.RoleMotion, livePhoto + "_2_101_o.mov", dto.RoleMotion},
	} {
		var got []string
		for _, f := range groups[uuid].Files {
			got = append(got, filepath.Base(f.Path), f.Role)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: %v, want %v (the main file first)", uuid[:1], got, want)
		}
	}

	for _, tc := range []struct {
		uuid string
		want Want
		file string
	}{
		{video, WantVideo, video + "_2_201_o.mov"},
		{video, WantVideoH264, video + "_2_4_o.mp4"},
		{video, WantVideoHover, video + "_2_4_o.mp4"},
		{video, WantImage, ""},
		{livePhoto, WantLiveMotion, livePhoto + "_2_101_a.mov"}, // the edit's motion wins
		{livePhoto, WantImage, livePhoto + "_1_102_o.jpeg"},
	} {
		if got := filepath.Base(Local(bundle, tc.uuid, tc.want)); (tc.file == "" && got != ".") || (tc.file != "" && got != tc.file) {
			t.Errorf("Local(%s, %d) = %s, want %q", tc.uuid[:1], tc.want, got, tc.file)
		}
	}
}

// Photos is asked for access only where a library is: the root itself, or one at
// its top
func TestHasLibrary(t *testing.T) {
	root := t.TempDir()
	if HasLibrary(root) {
		t.Error("an empty folder")
	}
	makeLibrary(t, root, nil)
	if !HasLibrary(root) || !HasLibrary(filepath.Join(root, "Photos Library.photoslibrary")) {
		t.Error("a library at the top, or the library itself")
	}
}
