package folder

import (
	"context"

	"perceptrail/gontroller/pkg/importer/flow"
	"perceptrail/gontroller/pkg/model/dto"
	"perceptrail/gontroller/pkg/providers"
)

// Provider: the plain folder. It claims everything (it is asked last), groups by
// names, and has nothing on demand: its files are all there is — the transcode
// renders for them.
type Provider struct {
	grouper *Grouper
}

var _ providers.Provider = (*Provider)(nil)

func New() *Provider { return &Provider{grouper: &Grouper{}} }

func (p *Provider) Name() string                               { return "folder" }
func (p *Provider) Claims(string) bool                         { return true }
func (p *Provider) Grouper() providers.Grouper                 { return p.grouper }
func (p *Provider) Regroup(string) (flow.FileGroup, bool)      { return flow.FileGroup{}, false }
func (p *Provider) Owns(*dto.ItemDto) bool                     { return false }
func (p *Provider) Levels(*dto.ItemDto) []string               { return nil }
func (p *Provider) Start(context.Context, providers.Refresher) {}

func (p *Provider) Rendition(*dto.ItemDto, string, providers.Options) (providers.Rendition, error) {
	return providers.Rendition{}, providers.ErrNoRendition
}
