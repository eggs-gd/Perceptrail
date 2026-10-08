package config

// Perceptors: the `perceptors` section, by perceptor name; a perceptor not listed
// runs and is shown
//
//	perceptors:
//	  exif_size:
//	    client: false   # runs, but no button in the gallery
//	  exif_geo:
//	    enabled: false  # not run, not shown
type Perceptors map[string]Perceptor

type Perceptor struct {
	// Run it in the import. Default: true
	Enabled *bool `yaml:"enabled"`
	// Give its view of the sheet to the client (a button in the gallery). Default: true
	Client *bool `yaml:"client"`
}

// Providers: the `providers` section, by name; a provider not listed is enabled
// (one that needs a server — Immich — only when its url is set)
//
//	providers:
//	  apple:
//	    enabled: false   # not in the chain: a Photos library is a plain folder then
//	  immich:
//	    url: http://immich:2283
//	    api_key_file: immich.key   # or the env IMMICH_API_KEY
type Providers map[string]Provider

type Provider struct {
	// In the chain. Default: true
	Enabled *bool `yaml:"enabled"`
	// The server of a library read through its API (Immich)
	URL string `yaml:"url"`
	// A file holding its API key, relative to the config's directory; the key itself
	// is never in the config
	APIKeyFile string `yaml:"api_key_file"`
}

func (s Perceptors) Enabled(name string) bool {
	p, ok := s[name]
	return !ok || p.Enabled == nil || *p.Enabled
}

// Client: enabled and given to the client
func (s Perceptors) Client(name string) bool {
	p, ok := s[name]
	return s.Enabled(name) && (!ok || p.Client == nil || *p.Client)
}

func (p Providers) Enabled(name string) bool {
	c, ok := p[name]
	return !ok || c.Enabled == nil || *c.Enabled
}
