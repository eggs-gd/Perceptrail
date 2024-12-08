package metadata

type PluginMetadata struct {
	Name        string
	Description string
	Version     string
}

func NewMetadata(name, description, version string) PluginMetadata {
	return PluginMetadata{
		Name:        name,
		Description: description,
		Version:     version,
	}
}
