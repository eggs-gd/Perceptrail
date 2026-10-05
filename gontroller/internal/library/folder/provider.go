package folder

import (
	"context"

	"perceptrail/gontroller/internal/library/provider"
	"perceptrail/gontroller/internal/model/dto"
)

// Provider: the plain folder. It claims everything (it is asked last), groups by
// names, and has nothing on demand: its files are all there is — the transcode
// renders for them.
type Provider struct {
	grouper *Grouper
}

var _ provider.Provider = (*Provider)(nil)

func New() *Provider { return &Provider{grouper: &Grouper{}} }

func (p *Provider) Claims(string) bool           { return true }
func (p *Provider) Grouper() provider.Grouper    { return p.grouper }
func (p *Provider) Skipped(string) []string      { return nil }
func (p *Provider) Owns(*dto.ItemDto) bool       { return false }
func (p *Provider) Levels(*dto.ItemDto) []string { return nil }
func (p *Provider) Start(context.Context)        {}

func (p *Provider) Rendition(*dto.ItemDto, string, provider.Options) (provider.Rendition, error) {
	return provider.Rendition{}, provider.ErrNoRendition
}
