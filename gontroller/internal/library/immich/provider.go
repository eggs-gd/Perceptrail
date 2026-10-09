// Package immich reads an Immich server as a library, through its API only and
// never writing to it: the walk lists the timeline's assets (search) as files of
// ours with URLs for paths (immich://…, see asset.go), the grouper makes each asset
// one group, and the routes serve those files through a proxy that adds the key.
// Nothing of it is on a disk, nothing of it is rendered by us: Immich's previews,
// thumbnails and playback are the renditions.
package immich

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
)

// Provider: an Immich server. It claims the paths it listed (immich://), lists them
// itself (the walk's Source) and serves them (File); it keeps no renditions to ask
// for on demand — its files are its renditions.
type Provider struct {
	client  *client
	index   *index
	grouper *Grouper
	logger  *l.Logger
}

var _ provider.Provider = (*Provider)(nil)

// New: server is the Immich URL (http://host:2283), key its API key (Key)
func New(server, key string, logger *l.Logger) (*Provider, error) {
	c, err := newClient(server, key)
	if err != nil {
		return nil, err
	}
	x := &index{assets: map[string]*asset{}}
	return &Provider{client: c, index: x, grouper: newGrouper(x), logger: logger}, nil
}

// Root: the prefix of its paths (the walk judges its deletions by it)
func (p *Provider) Root() string { return scheme }

// List: every file of the timeline's assets, page by page; an error — the listing
// is incomplete (nothing of it is gone this pass)
func (p *Provider) List(ctx context.Context, emit func(dto.ItemEntry) bool) error {
	p.index.reset()
	assets, cursor := 0, ""
	for {
		page, next, err := p.client.search(ctx, cursor)
		if err != nil {
			return err
		}
		for i := range page {
			a := &page[i]
			if !a.listed() {
				continue
			}
			p.index.put(a)
			assets++
			for _, f := range a.files() {
				entry := dto.ItemEntry{Path: f.path, Name: path.Base(f.path), Size: f.size, MimeType: f.mime, ModTime: a.UpdatedAt}
				if !emit(entry) {
					return ctx.Err()
				}
			}
		}
		if next == "" {
			p.logger.Info("Immich listed", l.Int("assets", assets))
			return nil
		}
		cursor = next
	}
}

func (p *Provider) Claims(path string) bool { return strings.HasPrefix(path, scheme) }

func (p *Provider) Grouper() provider.Grouper { return p.grouper }

func (p *Provider) Skipped(string) []string { return nil }

func (p *Provider) Owns(item *dto.ItemDto) bool { return strings.HasPrefix(item.Path, scheme) }

func (p *Provider) Levels(*dto.ItemDto) []string { return nil }

func (p *Provider) Rendition(*dto.ItemDto, string, provider.Options) (provider.Rendition, error) {
	return provider.Rendition{}, provider.ErrNoRendition
}

func (p *Provider) Start(context.Context) {}

// File: a path of ours served from Immich — the request (its Range too) passed on
// with the key, the answer as Immich gives it; nil when the path is not one of ours
func (p *Provider) File(path string) http.Handler {
	upstream := p.upstream(path)
	if upstream == nil {
		return nil
	}
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL, r.Out.Host = upstream, ""
			// The browser's own credentials are not Immich's business
			r.Out.Header.Del("Cookie")
			r.Out.Header.Del("Authorization")
			r.Out.Header.Set("x-api-key", p.client.key)
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			return nil
		},
		Transport: p.client.http.Transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.Context().Err() == nil {
				p.logger.Warn("Immich file not served", l.String("file", path), l.Error(err))
			}
			w.WriteHeader(http.StatusBadGateway)
		},
	}
}

// upstream: the API's URL of a path of ours (see asset.go), nil when it is none
func (p *Provider) upstream(path string) *url.URL {
	id, file, ok := strings.Cut(strings.TrimPrefix(path, scheme), "/")
	if !ok || !strings.HasPrefix(path, scheme) || !isID(id) {
		return nil
	}
	var api string
	query := url.Values{}
	switch {
	case strings.HasPrefix(file, "original/"):
		api = "/assets/" + id + "/original"
	case file == "preview.jpg":
		api, query = "/assets/"+id+"/thumbnail", url.Values{"size": {"preview"}}
	case file == "thumbnail.webp":
		api, query = "/assets/"+id+"/thumbnail", url.Values{"size": {"thumbnail"}}
	case file == "playback.mp4":
		api = "/assets/" + id + "/video/playback"
	case strings.HasPrefix(file, "motion/"):
		video := strings.TrimSuffix(strings.TrimPrefix(file, "motion/"), ".mp4")
		if !isID(video) {
			return nil
		}
		api = "/assets/" + video + "/video/playback"
	default:
		return nil
	}
	u := *p.client.api
	u.Path += api
	u.RawQuery = query.Encode()
	return &u
}

// isID: an Immich id (a UUID) — nothing else goes into an API path
func isID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'):
			return false
		}
	}
	return true
}
