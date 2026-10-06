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
//
//	providers:
//	  apple:
//	    enabled: false   # not in the chain: a Photos library is a plain folder then
type Providers map[string]struct {
	Enabled *bool `yaml:"enabled"`
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
