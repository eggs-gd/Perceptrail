package app

// LogCategory: the named logger of a part of the system (AppContext.Logger)
type LogCategory string

const (
	LogConfig   LogCategory = "config"
	LogPlugins  LogCategory = "plugins"
	LogDB       LogCategory = "db"
	LogHTTP     LogCategory = "http"
	LogImporter LogCategory = "importer"
)
