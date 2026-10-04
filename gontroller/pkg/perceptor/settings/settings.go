// Package settings: the config section of perceptors (`perceptors:` in config.yml),
// per perceptor by its name. Its own package: the plugin manager reads it, and app
// (which the manager imports) holds the config.
package settings

// Perceptors: by perceptor name; a perceptor not listed runs and is shown
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

func (s Perceptors) Enabled(name string) bool {
	p, ok := s[name]
	return !ok || p.Enabled == nil || *p.Enabled
}

// Client: enabled and given to the client
func (s Perceptors) Client(name string) bool {
	p, ok := s[name]
	return s.Enabled(name) && (!ok || p.Client == nil || *p.Client)
}
