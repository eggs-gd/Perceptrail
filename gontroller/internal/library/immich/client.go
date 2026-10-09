package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// client: the Immich API as the provider uses it — read only: the timeline's
// assets (search), their files (served through a proxy)
type client struct {
	api  *url.URL // the server's /api
	key  string
	http *http.Client
}

// searchPage: the assets a listing asks for at once (the API's maximum)
const searchPage = 1000

type searchRequest struct {
	Size     int          `json:"size"`
	WithExif bool         `json:"withExif"`
	Cursor   string       `json:"cursor,omitempty"`
	Filter   searchFilter `json:"filter"`
}

// searchFilter: what the library is — the timeline (not the archive, not hidden:
// a Live Photo's video is hidden, its photo brings it), not in the trash
type searchFilter struct {
	Visibility enumFilter `json:"visibility"`
	TrashedAt  nullFilter `json:"trashedAt"`
}

type enumFilter struct {
	Eq string `json:"eq"`
}

// nullFilter: {"eq": null} — the field is not set
type nullFilter struct {
	Eq *string `json:"eq"`
}

type searchResponse struct {
	Assets struct {
		Items      []asset `json:"items"`
		NextCursor *string `json:"nextCursor"`
	} `json:"assets"`
}

// Key: the API key — the env IMMICH_API_KEY, else the file's first line; never
// from the config itself
func Key(file string) (string, error) {
	if key := strings.TrimSpace(os.Getenv("IMMICH_API_KEY")); key != "" {
		return key, nil
	}
	if file == "" {
		return "", fmt.Errorf("immich: no API key (api_key_file or IMMICH_API_KEY)")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("immich: API key: %w", err)
	}
	key, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
	if key = strings.TrimSpace(key); key == "" {
		return "", fmt.Errorf("immich: API key file %s is empty", file)
	}
	return key, nil
}

// newClient: server is the Immich URL as its users open it (http://host:2283); the
// API is under /api
func newClient(server, key string) (*client, error) {
	base, err := url.Parse(strings.TrimRight(server, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("immich: url %q is not a server's URL", server)
	}
	if !strings.HasSuffix(base.Path, "/api") {
		base.Path += "/api"
	}
	// No overall timeout: a video streams for as long as it plays; a server that
	// does not answer at all is cut by the headers' wait
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = time.Minute
	return &client{api: base, key: key, http: &http.Client{Transport: transport}}, nil
}

// search: one page of the timeline's assets with their EXIF, after cursor ("" the
// first); next is "" after the last page
func (c *client) search(ctx context.Context, cursor string) (assets []asset, next string, err error) {
	body, err := json.Marshal(searchRequest{
		Size: searchPage, WithExif: true, Cursor: cursor,
		Filter: searchFilter{Visibility: enumFilter{Eq: "timeline"}, TrashedAt: nullFilter{}},
	})
	if err != nil {
		return nil, "", err
	}
	req, err := c.request(ctx, http.MethodPost, "/search/metadata", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", fmt.Errorf("immich: search: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", fmt.Errorf("immich: search: %w", err)
	}
	if out.Assets.NextCursor != nil {
		next = *out.Assets.NextCursor
	}
	return out.Assets.Items, next, nil
}

// request: a call of the API with the key
func (c *client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u := *c.api
	u.Path += path
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("Accept", "application/json")
	return req, nil
}
