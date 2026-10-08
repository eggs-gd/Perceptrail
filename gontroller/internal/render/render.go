// Package render: the expensive stage — renditions of a plain folder's items, taken
// from the work queue at their own pace, beside the import (which never waits for
// it and knows nothing of it). The database says what is due; ItemPublished only
// wakes it.
//
//	wake → feed (Due, Take) → workers × N (render) → Finish | Fail
//
// A photo: its stills (photo.go, libvips). A video: a video every browser plays and
// its poster's stills (video.go, ffmpeg). A Live Photo: its photo's stills and its
// motion's video.
//
// Not obvious:
//   - A library's items (Apple Photos) are rendered by their library: render finishes
//     them with no renditions, so they are not due again for this version.
//   - The version is what the renditions are — sizes, format, quality — not what made
//     them: another config makes every item due again, lazily, page by page; a new
//     libvips changes nothing. The sweep (sweep.go) takes away what is no longer
//     listed.
//   - No vipsthumbnail or no ffmpeg (not found, does not run): render does not start
//     — a tool missing is not a failure of every item, nor a Live Photo rendered
//     without its motion for good.
//   - A safety net wakes it every minute too: a missed event costs a minute, never
//     an item (the event is a hint, not the truth).
package render

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"perceptrail/gontroller/internal/cache"
	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/library"
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
)

// Config: what render reads of the config
type Config interface {
	CacheDir() string
	Render() config.Render
}

// Store: what render asks of the model — the queue, and the event that wakes it
type Store interface {
	Due(slug, version string, after dto.Cursor, n int) ([]*dto.ItemDto, dto.Cursor, error)
	Take(slug string, guids []api.GUID) ([]dto.Taken, error)
	Renew(slug string, guid api.GUID, lease int64) (bool, error)
	Finish(done dto.WorkDone) (bool, error)
	Fail(failed dto.WorkFailed) error
	Prune() (int64, error)
	GetLinkedFiles(guid api.GUID) ([]*dto.FileDto, error)
	RenditionPaths() ([]string, error)
	Published() *pubsub.Topic[dto.ItemPublished]
}

// Service: the render service, one per server
type Service struct {
	cfg     Config
	db      Store
	logger  *l.Logger
	version string        // what the renditions are: sizes, format, quality, the video's size
	ffmpeg  *ffmpeg       // the video tools (found at Start)
	wake    chan struct{} // one pending wake-up: many events, one pass
}

// job: an item taken, with the lease its result carries
type job struct {
	item  *dto.ItemDto
	lease int64
}

const (
	slug    = "render"
	page    = 64              // items asked of the queue at once
	recheck = time.Minute     // the safety net between wake-ups
	timeout = 2 * time.Minute // a photo's renditions at most
	renew   = 5 * time.Minute // a lease renewed this often while an item renders
)

func New(cfg Config, db Store, logger *l.Logger) *Service {
	return &Service{cfg: cfg, db: db, logger: logger, version: versionOf(cfg.Render()), wake: make(chan struct{}, 1)}
}

// Start renders what is due until ctx ends: a pass, then a wait for an item
// published or the safety net; off in the config — it returns at once
func (s *Service) Start(ctx context.Context) {
	if !s.cfg.Render().Enabled() {
		return
	}
	cfg := s.cfg.Render()
	if err := runs(ctx, cfg.Vipsthumbnail); err != nil {
		s.logger.Error("Render off: no vipsthumbnail", l.String("vipsthumbnail", cfg.Vipsthumbnail), l.Error(err))
		return
	}
	video, err := findFFmpeg(ctx, cfg.FFmpeg, cfg.FFprobe())
	if err != nil {
		s.logger.Error("Render off: no ffmpeg", l.String("ffmpeg", cfg.FFmpeg), l.Error(err))
		return
	}
	if video.tonemap == "" {
		s.logger.Warn("Render: this ffmpeg has no zscale — HDR videos are kept as they are", l.String("ffmpeg", cfg.FFmpeg))
	}
	s.ffmpeg = video
	published := s.db.Published().Subscribe(func(dto.ItemPublished) {
		select {
		case s.wake <- struct{}{}:
		default: // already woken: this item is in the pass to come
		}
	})
	defer published.Close()

	jobs := make(chan job)
	free := make(chan struct{}, s.cfg.Render().Workers) // a place per worker at work
	var workers sync.WaitGroup
	for range s.cfg.Render().Workers {
		workers.Go(func() {
			for j := range jobs {
				s.render(ctx, j)
				<-free
			}
		})
	}
	defer workers.Wait()
	defer close(jobs)

	safety, sweeper := time.NewTicker(recheck), time.NewTicker(sweepEvery)
	defer safety.Stop()
	defer sweeper.Stop()
	s.sweep()
	for {
		s.pass(ctx, jobs, free)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-safety.C:
		case <-sweeper.C:
			s.sweep()
		}
	}
}

// pass: every item due now, page by page, to the workers; a library's items are
// finished as nothing to render. An item is taken only once a worker is free for it
// (free: a place per worker): a lease starts when its work does, never while it
// queues behind a page of long videos.
func (s *Service) pass(ctx context.Context, jobs chan<- job, free chan struct{}) {
	var cursor dto.Cursor
	for !cursor.Over() && ctx.Err() == nil {
		due, next, err := s.db.Due(slug, s.version, cursor, page)
		if err != nil {
			s.logger.Error("Render: the queue not read", l.Error(err))
			return
		}
		cursor = next
		for _, item := range due {
			if library.Of(item) != nil || !picture(item) && !moving(item) {
				s.finish(dto.WorkDone{Slug: slug, GUID: item.GUID, Version: s.version, Input: item.HashShort})
				continue
			}
			select {
			case free <- struct{}{}: // a worker is free for it
			case <-ctx.Done():
				return
			}
			taken, err := s.db.Take(slug, []api.GUID{item.GUID})
			if err != nil || len(taken) == 0 {
				<-free
				if err != nil {
					s.logger.Error("Render: an item not taken", l.Error(err))
					return
				}
				continue // another holds it
			}
			jobs <- job{item, taken[0].Lease}
		}
	}
}

// render: one item's renditions, finished or failed under its lease — renewed while
// it works (a video may take longer than a lease); a lease lost stops it
func (s *Service) render(ctx context.Context, j job) {
	item := j.item
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.keep(ctx, cancel, j)
	renditions, err := s.renditions(ctx, item)
	if err != nil {
		s.logger.Warn("Render failed", l.String("file", item.Path), l.Error(err))
		failed := dto.WorkFailed{Slug: slug, GUID: item.GUID, Lease: j.lease, Version: s.version, Input: item.HashShort, Err: err.Error()}
		if err := s.db.Fail(failed); err != nil {
			s.logger.Error("Render: a failure not kept", l.String("file", item.Path), l.Error(err))
		}
		return
	}
	s.finish(dto.WorkDone{Slug: slug, GUID: item.GUID, Lease: j.lease, Version: s.version, Input: item.HashShort, Renditions: renditions})
}

func (s *Service) finish(done dto.WorkDone) {
	if _, err := s.db.Finish(done); err != nil {
		s.logger.Error("Render: a result not kept", l.String("guid", done.GUID.String()), l.Error(err))
	}
}

// keep: the job's lease renewed until ctx ends; lost — cancel stops the work (its
// result would be dropped anyway)
func (s *Service) keep(ctx context.Context, cancel context.CancelFunc, j job) {
	tick := time.NewTicker(renew)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			held, err := s.db.Renew(slug, j.item.GUID, j.lease)
			if err != nil {
				s.logger.Error("Render: a lease not renewed", l.String("file", j.item.Path), l.Error(err))
				continue
			}
			if !held {
				s.logger.Warn("Render: a lease lost, the work stops", l.String("file", j.item.Path))
				cancel()
				return
			}
		}
	}
}

// renditions: an item's renditions by what it is — a photo, a video, a Live Photo
// (a photo with a motion file in its group)
func (s *Service) renditions(ctx context.Context, item *dto.ItemDto) ([]dto.RenditionDto, error) {
	cfg := s.cfg.Render()
	rel := cache.ItemDir("r", item.GUID)
	dir := filepath.Join(s.cfg.CacheDir(), rel)
	still := func(ctx context.Context, src string) ([]dto.RenditionDto, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return photo(ctx, cfg.Vipsthumbnail, cfg.Format, cfg.Sizes, item.GUID, src, dir, rel)
	}
	if moving(item) {
		return s.ffmpeg.video(ctx, item.GUID, item.Path, cfg.Video, dir, rel, true, still)
	}
	stills, err := still(ctx, item.Path)
	if err != nil {
		return nil, err
	}
	files, err := s.db.GetLinkedFiles(item.GUID)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.Role == dto.RoleMotion { // a Live Photo: its video too
			motion, err := s.ffmpeg.video(ctx, item.GUID, f.Path, cfg.Video, dir, rel, false, still)
			if err != nil {
				return nil, err
			}
			return append(stills, motion...), nil
		}
	}
	return stills, nil
}

// versionOf: what the renditions are — sizes, format, quality, the video's size
// ("400_1600-webp-q80-v1280")
func versionOf(cfg config.Render) string {
	sizes := make([]string, len(cfg.Sizes))
	for i, size := range cfg.Sizes { // sorted by the config
		sizes[i] = strconv.Itoa(size)
	}
	return fmt.Sprintf("%s-%s-q%d-v%d", strings.Join(sizes, "_"), cfg.Format, quality, cfg.Video)
}

// picture: the item's main file is a picture
func picture(item *dto.ItemDto) bool { return strings.HasPrefix(item.MimeType, "image/") }

// moving: the item's main file is a video
func moving(item *dto.ItemDto) bool { return strings.HasPrefix(item.MimeType, "video/") }
