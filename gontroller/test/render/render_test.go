// The render service over a real model, real files and a real vipsthumbnail: an item
// published wakes it, its renditions land in the cache and make it Ready; a video
// waits for a video renderer; a file that cannot be rendered is kept as a failure,
// not retried at once; the original is never touched
package render_test

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/model"
	"perceptrail/gontroller/internal/model/dto"
	"perceptrail/gontroller/internal/render"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
	"github.com/eggs-gd/perceplib/api"
)

// version: what the default settings make — sizes, format, quality
const version = "400_1600-webp-q80-v1280"

// start: a model and a running render service over a temporary data dir
func start(t *testing.T) (*model.Proxy, *config.Config) {
	t.Helper()
	return startAfter(t, nil)
}

// startAfter: start, with before run on the model and the config first (what the
// service finds at its start)
func startAfter(t *testing.T, before func(*model.Proxy, *config.Config)) (*model.Proxy, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	conf := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(conf, []byte("render:\n  workers: 2\n  ffmpeg: "+ffmpegBin()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(conf)
	if err != nil {
		t.Fatal(err)
	}
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	db, err := model.Open(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if before != nil {
		before(db, cfg)
	}
	ctx, stop := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { render.New(cfg, db, logger).Start(ctx); close(stopped) }()
	t.Cleanup(func() { stop(); <-stopped; db.Close() })
	return db, cfg
}

// publish: a plain folder's item of this file, through the cheap stage
func publish(t *testing.T, db *model.Proxy, path, mime string) *dto.ItemDto {
	t.Helper()
	f, err := db.CreateFile(dto.ItemEntry{Path: path, Name: filepath.Base(path), MimeType: mime})
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateItem(f)
	if err != nil {
		t.Fatal(err)
	}
	item.HashShort = "fingerprint"
	if _, err := db.Publish(item); err != nil {
		t.Fatal(err)
	}
	return item
}

// photo: a JPEG of w × h
func photo(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		img.Set(x, x*h/w, color.RGBA{200, 80, 40, 255})
	}
	path := filepath.Join(t.TempDir(), "photo.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

// work: the item's render row once render has written its outcome
func work(t *testing.T, db *model.Proxy, item *dto.ItemDto) *dto.WorkDto {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if row, err := db.Work(item.GUID, "render"); err == nil && (row.DoneAt != 0 || row.Error != "") {
			return row
		}
	}
	t.Fatal("render wrote nothing within 10 s")
	return nil
}

// A photo: a rendition per size, never upscaled (800 × 600: 400 and its own size),
// in the cache, and the item Ready
func TestRenderPhoto(t *testing.T) {
	db, cfg := start(t)
	item := publish(t, db, photo(t, 800, 600), "image/jpeg")
	if row := work(t, db, item); row.DoneAt == 0 || row.Version != version {
		t.Fatalf("row %+v", row)
	}
	rs, err := db.Renditions(item.GUID)
	if err != nil || len(rs) != 2 {
		t.Fatalf("renditions %+v %v", rs, err)
	}
	if rs[0].W != 400 || rs[0].H != 300 || rs[1].W != 800 || rs[1].H != 600 || rs[0].Format != "webp" {
		t.Errorf("sizes %+v", rs)
	}
	for _, r := range rs {
		if info, err := os.Stat(filepath.Join(cfg.CacheDir(), r.Path)); err != nil || info.Size() != r.Bytes {
			t.Errorf("%s: %v", r.Path, err)
		}
	}
	if it, _ := db.GetItemByGUID(item.GUID); it.State != dto.Ready {
		t.Errorf("state %v, want Ready", it.State)
	}
}

// A HEIC (what Chrome does not show): rendered the same — when this libvips can
// write one to test with
func TestRenderHEIC(t *testing.T) {
	src := photo(t, 1200, 900)
	heic := filepath.Join(t.TempDir(), "photo.heic")
	if out, err := exec.Command("vips", "copy", src, heic).CombinedOutput(); err != nil {
		t.Skipf("this libvips writes no HEIC: %s", out)
	}
	db, _ := start(t)
	item := publish(t, db, heic, "image/heic")
	if row := work(t, db, item); row.DoneAt == 0 {
		t.Fatalf("row %+v", row)
	}
	if rs, _ := db.Renditions(item.GUID); len(rs) != 2 || rs[0].W != 400 || rs[0].H != 300 || rs[1].W != 1200 {
		t.Errorf("renditions %+v: 400 and its own 1200", rs)
	}
}

// A file vipsthumbnail cannot read: a failure, one attempt, backed off — not due
// again at once
func TestRenderFails(t *testing.T) {
	db, _ := start(t)
	broken := filepath.Join(t.TempDir(), "broken.jpg")
	if err := os.WriteFile(broken, []byte("not a jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := publish(t, db, broken, "image/jpeg")
	row := work(t, db, item)
	if row.Error == "" || row.Attempts != 1 || row.DoneAt != 0 || row.LeaseUntil != 0 {
		t.Errorf("row %+v: one attempt, its error, no lease", row)
	}
	if due, _, _ := db.Due("render", row.Version, dto.Cursor{}, 10); len(due) != 0 {
		t.Errorf("due again right after its failure: %d", len(due))
	}
}

// The original is only read: no link to it, nothing written next to it
func TestRenderLeavesTheOriginal(t *testing.T) {
	db, _ := start(t)
	src := photo(t, 640, 480)
	before := links(t, src)
	entries, _ := os.ReadDir(filepath.Dir(src))
	item := publish(t, db, src, "image/jpeg")
	work(t, db, item)
	if after := links(t, src); after != before {
		t.Errorf("the original's link count %d → %d", before, after)
	}
	if now, _ := os.ReadDir(filepath.Dir(src)); len(now) != len(entries) {
		t.Errorf("files next to the original: %d → %d", len(entries), len(now))
	}
}

func links(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return uint64(info.Sys().(*syscall.Stat_t).Nlink)
}

// The sweep at start: a file the database no longer lists goes, with the directories
// it leaves empty; a file it lists stays, and so does a fresh one (a worker may be
// writing it)
func TestSweep(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour)
	write := func(cache, rel string, at time.Time) string {
		t.Helper()
		path := filepath.Join(cache, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte("webp"), 0o644); err != nil {
			t.Fatal(err)
		}
		for p := path; p != cache; p = filepath.Dir(p) {
			os.Chtimes(p, at, at)
		}
		return path
	}
	var stale, fresh, listed string
	_, cfg := startAfter(t, func(db *model.Proxy, cfg *config.Config) {
		stale = write(cfg.CacheDir(), "r/aa/bb/AABB-old/vips1-400_1600-webp/400.webp", old) // an old layout
		fresh = write(cfg.CacheDir(), "r/cc/dd/CCDD-new/400.webp", time.Now())
		f, _ := db.CreateFile(dto.ItemEntry{Path: "/x.jpg", Name: "x.jpg"})
		item, _ := db.CreateItem(f)
		item.HashShort = "h"
		db.UpdateItem(item)
		rel := filepath.Join("r", "ee", "ff", string(item.GUID), "400.webp")
		listed = write(cfg.CacheDir(), rel, old)
		taken, _ := db.Take("render", []api.GUID{item.GUID})
		if done, err := db.Finish(dto.WorkDone{Slug: "render", GUID: item.GUID, Lease: taken[0].Lease, Version: "v0", Input: "h",
			Renditions: []dto.RenditionDto{{GUID: item.GUID, Size: 400, Format: "webp", Path: rel}}}); err != nil || !done {
			t.Fatalf("finish: %v %v", done, err)
		}
	})
	gone := func(path string) bool { _, err := os.Stat(path); return os.IsNotExist(err) }
	for deadline := time.Now().Add(5 * time.Second); !gone(stale) && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
	}
	if !gone(stale) || !gone(filepath.Join(cfg.CacheDir(), "r", "aa")) {
		t.Error("the stale rendition or its directories stayed")
	}
	if gone(fresh) || gone(listed) {
		t.Errorf("swept what stays: fresh gone %v, listed gone %v", gone(fresh), gone(listed))
	}
}

// An original exactly as big as a size: one rendition — the next size would be the
// same image again (two equal widths in a srcset)
func TestRenderExactSize(t *testing.T) {
	db, _ := start(t)
	item := publish(t, db, photo(t, 400, 300), "image/jpeg")
	if row := work(t, db, item); row.DoneAt == 0 {
		t.Fatalf("row %+v", row)
	}
	if rs, _ := db.Renditions(item.GUID); len(rs) != 1 || rs[0].W != 400 {
		t.Errorf("renditions %+v, want one of 400", rs)
	}
}

// No vipsthumbnail: render does not start — no item fails for it (five failures
// would stop an item until a new version)
func TestRenderWithoutVips(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(conf, []byte("render:\n  vipsthumbnail: /nowhere/vipsthumbnail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(conf)
	if err != nil {
		t.Fatal(err)
	}
	logger := l.NewLogger(l.FatalLevel, &tree.Decorator{})
	db, err := model.Open(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	stopped := make(chan struct{})
	go func() { render.New(cfg, db, logger).Start(ctx); close(stopped) }()
	item := publish(t, db, photo(t, 800, 600), "image/jpeg")
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("render ran without its tool")
	}
	if row, err := db.Work(item.GUID, "render"); err == nil {
		t.Errorf("the item got a row without a tool to render it: %+v", row)
	}
}

// ffmpegBin: the ffmpeg the tests run (FFMPEG, e.g. one with zscale; else PATH's)
func ffmpegBin() string {
	if bin := os.Getenv("FFMPEG"); bin != "" {
		return bin
	}
	return "ffmpeg"
}

// clip: a test video of w × h, secs long, with sound; hlg: tagged HDR (HLG)
func clip(t *testing.T, w, h int, secs float64, hlg bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.mov")
	args := []string{"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=%dx%d:rate=30", w, h),
		"-f", "lavfi", "-i", "sine=frequency=440", "-t", strconv.FormatFloat(secs, 'f', 1, 64),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac"}
	if hlg { // the tags on the frames: the encoder writes those
		args = append(args, "-vf", "setparams=color_trc=arib-std-b67:color_primaries=bt2020:colorspace=bt2020nc")
	}
	if out, err := exec.Command(ffmpegBin(), append(args, path)...).CombinedOutput(); err != nil {
		t.Fatalf("a test clip: %v %s", err, out)
	}
	return path
}

// probed: a rendition's codecs and size as ffprobe sees them
func probed(t *testing.T, path string) string {
	t.Helper()
	probe := "ffprobe"
	if bin := os.Getenv("FFMPEG"); bin != "" {
		probe = filepath.Join(filepath.Dir(bin), "ffprobe")
	}
	out, err := exec.Command(probe, "-v", "error", "-show_entries", "stream=codec_name,width,height,color_transfer",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// byRole: renditions of a role
func byRole(rs []dto.RenditionDto, role string) []dto.RenditionDto {
	var out []dto.RenditionDto
	for _, r := range rs {
		if r.Role == role {
			out = append(out, r)
		}
	}
	return out
}

// A video: one every browser plays (H.264 and AAC, 1280 × 720 from 1920 × 1080) and
// its poster's stills; the item Ready
func TestRenderVideo(t *testing.T) {
	db, cfg := start(t)
	item := publish(t, db, clip(t, 1920, 1080, 2, false), "video/quicktime")
	if row := work(t, db, item); row.DoneAt == 0 {
		t.Fatalf("row %+v", row)
	}
	rs, _ := db.Renditions(item.GUID)
	motion, stills := byRole(rs, dto.RoleMotion), byRole(rs, dto.RoleStill)
	if len(motion) != 1 || motion[0].W != 1280 || motion[0].H != 720 || motion[0].Format != "mp4" {
		t.Fatalf("motion %+v", motion)
	}
	if got := probed(t, filepath.Join(cfg.CacheDir(), motion[0].Path)); !strings.Contains(got, "h264,1280,720") || !strings.Contains(got, "aac") {
		t.Errorf("the video: %s", got)
	}
	if len(stills) != 2 || stills[0].W != 400 || stills[1].W != 1600 {
		t.Errorf("the poster's stills %+v", stills)
	}
	if it, _ := db.GetItemByGUID(item.GUID); it.State != dto.Ready {
		t.Errorf("state %v, want Ready", it.State)
	}
}

// A small video is not upscaled
func TestRenderSmallVideo(t *testing.T) {
	db, _ := start(t)
	item := publish(t, db, clip(t, 640, 360, 1, false), "video/mp4")
	work(t, db, item)
	rs, _ := db.Renditions(item.GUID)
	if motion := byRole(rs, dto.RoleMotion); len(motion) != 1 || motion[0].W != 640 || motion[0].H != 360 {
		t.Errorf("motion %+v", motion)
	}
}

// An HDR (HLG) video renders — tone mapped to SDR and tagged so when this ffmpeg has
// zscale; without, it stays HLG and says so (never HDR samples tagged SDR)
func TestRenderHDRVideo(t *testing.T) {
	db, cfg := start(t)
	item := publish(t, db, clip(t, 1280, 720, 1, true), "video/quicktime")
	if row := work(t, db, item); row.DoneAt == 0 {
		t.Fatalf("row %+v", row)
	}
	rs, _ := db.Renditions(item.GUID)
	motion := byRole(rs, dto.RoleMotion)
	if len(motion) != 1 {
		t.Fatalf("renditions %+v", rs)
	}
	filters, _ := exec.Command(ffmpegBin(), "-hide_banner", "-filters").Output()
	want := "arib-std-b67"
	if strings.Contains(string(filters), " zscale ") {
		want = "bt709"
	}
	if got := probed(t, filepath.Join(cfg.CacheDir(), motion[0].Path)); !strings.Contains(got, want) {
		t.Errorf("the video's transfer: %s, want %s", got, want)
	}
}

// A Live Photo (a photo with a motion file in its group): its photo's stills and its
// motion's video — no poster
func TestRenderLivePhoto(t *testing.T) {
	db, _ := start(t)
	still := photo(t, 1200, 900)
	f, err := db.CreateFile(dto.ItemEntry{Path: still, Name: "live.jpg", MimeType: "image/jpeg"})
	if err != nil {
		t.Fatal(err)
	}
	motion, err := db.CreateFile(dto.ItemEntry{Path: clip(t, 1280, 960, 1, false), Name: "live.mov", MimeType: "video/quicktime"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateItem(f)
	if err != nil {
		t.Fatal(err)
	}
	f.LinkToItem(item.GUID)
	f.Role = dto.RoleOriginal
	motion.LinkToItem(item.GUID)
	motion.Role = dto.RoleMotion
	db.UpdateFiles([]*dto.FileDto{f, motion})
	item.HashShort = "fingerprint"
	if _, err := db.Publish(item); err != nil {
		t.Fatal(err)
	}
	if row := work(t, db, item); row.DoneAt == 0 {
		t.Fatalf("row %+v", row)
	}
	rs, _ := db.Renditions(item.GUID)
	if stills, moving := byRole(rs, dto.RoleStill), byRole(rs, dto.RoleMotion); len(stills) != 2 || stills[1].W != 1200 || len(moving) != 1 || moving[0].W != 1280 {
		t.Errorf("stills %+v, motion %+v", stills, moving)
	}
}

// No ffmpeg: render does not start — no item rendered without its video, for good
func TestRenderWithoutFFmpeg(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(conf, []byte("render:\n  ffmpeg: /nowhere/ffmpeg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(conf)
	if err != nil {
		t.Fatal(err)
	}
	logger := l.NewLogger(l.FatalLevel, &tree.Decorator{})
	db, err := model.Open(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	stopped := make(chan struct{})
	go func() { render.New(cfg, db, logger).Start(ctx); close(stopped) }()
	item := publish(t, db, photo(t, 800, 600), "image/jpeg")
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("render ran without ffmpeg")
	}
	if row, err := db.Work(item.GUID, "render"); err == nil {
		t.Errorf("the item got a row without the tools to render it: %+v", row)
	}
}
