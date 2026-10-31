package immich

import (
	"strings"
	"sync"

	"perceptrail/gontroller/internal/model/dto"

	"github.com/eggs-gd/perceplib/api"

	chain "github.com/eggs-gd/go-chain"
)

// Grouper: an Immich asset's files into one group, as the listing gave them — a
// group goes out when its last file has arrived; a file the walk found missing
// passes through as it is (an asset of its own). The key is the asset's id (the
// item's GUID), the main file the original; Immich's metadata wins over anything
// read, its checksum is the fingerprint.
type Grouper struct {
	index   *index
	arrived map[string]map[string]*dto.FileDto // by asset id, by path
}

// index: the assets of the listing under way, by id — the listing (the walk's
// goroutine) fills it before it sends an asset's files, the grouper reads it
type index struct {
	mu     sync.Mutex
	assets map[string]*asset
}

// Flush: the walk ended. Groups that did not complete (a file of an asset changed
// in Immich during the walk) wait for the next walk.
func (g *Grouper) Flush() ([]dto.Asset, error) {
	g.arrived = map[string]map[string]*dto.FileDto{}
	return nil, nil
}

func (g *Grouper) Decorate(walked dto.WalkedFile) (dto.Asset, error) {
	f := walked.FileDto
	if walked.Missing {
		return dto.Asset{Missing: []*dto.FileDto{f}}, nil
	}
	id := assetID(f.Path)
	a := g.index.get(id)
	if a == nil {
		return dto.Asset{}, chain.ErrSkippedItem // not listed this pass
	}
	files := a.files()
	got := g.arrived[id]
	if got == nil {
		got = map[string]*dto.FileDto{}
		g.arrived[id] = got
	}
	got[f.Path] = f
	if len(got) < len(files) {
		return dto.Asset{}, chain.ErrSkippedItem // not complete yet
	}
	delete(g.arrived, id)
	return group(a, files, got), nil
}

func newGrouper(index *index) *Grouper {
	return &Grouper{index: index, arrived: map[string]map[string]*dto.FileDto{}}
}

// group: the asset's rows in its files' order, with what Immich says of them
func group(a *asset, files []file, rows map[string]*dto.FileDto) dto.Asset {
	kind := a.kind()
	g := dto.Asset{
		Key: api.GUID(a.ID), Meta: a.meta(), Kind: kind, Fingerprint: "immich:" + a.Checksum,
		// The rows' stat is the asset's updatedAt: a change in Immich is a changed
		// file already; this only tells the gate the kind changed with it
		MetaHash: kind + "@" + a.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000Z"),
	}
	for _, file := range files {
		row := rows[file.path]
		row.SetRole(file.role)
		row.Width, row.Height, row.Codec = file.w, file.h, file.codec
		g.Files = append(g.Files, row)
		if file.role == dto.RoleStill {
			g.Show = append(g.Show, row) // the preview first, then the thumbnail
		}
	}
	return g
}

// assetID: the id a path of ours is under (immich://<id>/…)
func assetID(path string) string {
	id, _, _ := strings.Cut(strings.TrimPrefix(path, scheme), "/")
	return id
}

func (x *index) reset() {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.assets = map[string]*asset{}
}

func (x *index) put(a *asset) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.assets[a.ID] = a
}

func (x *index) get(id string) *asset {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.assets[id]
}
