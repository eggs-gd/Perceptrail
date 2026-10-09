package identify

import (
	l "github.com/eggs-gd/go-zap-decor"
)

// show: the show step's logic — what the client may show of the identified asset:
// every file's pixels and codec (stored), what to show now (preview); then the
// item leaves the stage. The last step to use the stage's exiftool: it closes it.
type show struct {
	db     SizesStore
	tool   Exiftool
	dir    string // the data's cache: extracted previews go under previews/ (cache.ItemDir)
	logger *l.Logger
}

func (s *show) Decorate(d *draft) (*Item, error) {
	setSizes(d)
	if _, err := s.db.UpdateFiles(d.Files); err != nil {
		return nil, err
	}
	d.Item.PreviewPath, d.Item.PreviewMime = preview(d, s.tool, s.dir, s.logger)
	d.Item.PreviewColor = previewColor(d.Item.GUID.String(), d.Item.PreviewPath, d.Item.PreviewMime, s.logger)
	return yield(d), nil
}

func (s *show) Stop() { s.tool.Close() }

func newShow(db SizesStore, tool Exiftool, cacheDir string, logger *l.Logger) *show {
	return &show{db: db, tool: tool, dir: cacheDir, logger: logger}
}
