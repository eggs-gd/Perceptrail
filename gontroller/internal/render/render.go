// Package render: the expensive stage — renditions of a plain folder's items, taken
// from the work queue at their own pace, beside the import (which never waits for
// it and knows nothing of it). The database says what is due; ItemPublished only
// wakes it.
//
//	wake → feed (Due, Take) → workers × N (render) → Finish | Fail
//
// Not obvious:
//   - A library's items (Apple Photos) are rendered by their library: render
//     finishes them with no renditions, so they are not due again for this version.
//   - A safety net wakes it every minute too: a missed event costs a minute, never
//     an item (the event is a hint, not the truth).
package render

import (
	"context"
	"sync"
	"time"

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
	Finish(done dto.WorkDone) (bool, error)
	Fail(failed dto.WorkFailed) error
	Published() *pubsub.Topic[dto.ItemPublished]
}

// Service: the render service, one per server
type Service struct {
	cfg    Config
	db     Store
	logger *l.Logger
	wake   chan struct{} // one pending wake-up: many events, one pass
}

// job: an item taken, with the lease its result carries
type job struct {
	item  *dto.ItemDto
	lease int64
}

const (
	slug    = "render"
	page    = 64          // items asked of the queue at once
	recheck = time.Minute // the safety net between wake-ups
)

func New(cfg Config, db Store, logger *l.Logger) *Service {
	return &Service{cfg: cfg, db: db, logger: logger, wake: make(chan struct{}, 1)}
}

// Start renders what is due until ctx ends: a pass, then a wait for an item
// published or the safety net; off in the config — it returns at once
func (s *Service) Start(ctx context.Context) {
	if !s.cfg.Render().Enabled {
		return
	}
	published := s.db.Published().Subscribe(func(dto.ItemPublished) {
		select {
		case s.wake <- struct{}{}:
		default: // already woken: this item is in the pass to come
		}
	})
	defer published.Close()

	jobs := make(chan job)
	var workers sync.WaitGroup
	for range s.cfg.Render().Workers {
		workers.Go(func() {
			for j := range jobs {
				s.render(j)
			}
		})
	}
	defer workers.Wait()
	defer close(jobs)

	safety := time.NewTicker(recheck)
	defer safety.Stop()
	for {
		s.pass(ctx, jobs)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-safety.C:
		}
	}
}

// pass: every item due now, page by page, to the workers; a library's items are
// finished as nothing to render
func (s *Service) pass(ctx context.Context, jobs chan<- job) {
	var cursor dto.Cursor
	for !cursor.Over() && ctx.Err() == nil {
		due, next, err := s.db.Due(slug, version, cursor, page)
		if err != nil {
			s.logger.Error("Render: the queue not read", l.Error(err))
			return
		}
		cursor = next
		ours := map[api.GUID]*dto.ItemDto{}
		var guids []api.GUID
		for _, item := range due {
			if library.Of(item) != nil {
				s.finish(dto.WorkDone{Slug: slug, GUID: item.GUID, Version: version, Input: item.HashShort})
				continue
			}
			ours[item.GUID] = item
			guids = append(guids, item.GUID)
		}
		if len(guids) == 0 {
			continue
		}
		taken, err := s.db.Take(slug, guids)
		if err != nil {
			s.logger.Error("Render: items not taken", l.Error(err))
			return
		}
		for _, t := range taken {
			select {
			case jobs <- job{ours[t.GUID], t.Lease}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// render: one item's renditions, finished or failed under its lease
func (s *Service) render(j job) {
	item := j.item
	renditions, err := standIn(s.cfg.CacheDir(), item)
	if err != nil {
		s.logger.Warn("Render failed", l.String("file", item.Path), l.Error(err))
		failed := dto.WorkFailed{Slug: slug, GUID: item.GUID, Lease: j.lease, Version: version, Input: item.HashShort, Err: err.Error()}
		if err := s.db.Fail(failed); err != nil {
			s.logger.Error("Render: a failure not kept", l.String("file", item.Path), l.Error(err))
		}
		return
	}
	s.finish(dto.WorkDone{Slug: slug, GUID: item.GUID, Lease: j.lease, Version: version, Input: item.HashShort, Renditions: renditions})
}

func (s *Service) finish(done dto.WorkDone) {
	if _, err := s.db.Finish(done); err != nil {
		s.logger.Error("Render: a result not kept", l.String("guid", done.GUID.String()), l.Error(err))
	}
}
