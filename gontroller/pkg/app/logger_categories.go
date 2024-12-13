package app

// LogCategory визначає категорії для логування різних компонентів системи
type LogCategory string

const (
	// Core categories
	LogCore      LogCategory = "core"
	LogConfig    LogCategory = "config"
	LogPlugins   LogCategory = "plugins"
	LogDB        LogCategory = "db"
	LogCache     LogCategory = "cache"
	LogHTTP      LogCategory = "http"
	LogWebsocket LogCategory = "ws"
	LogScheduler LogCategory = "scheduler"
	LogMetrics   LogCategory = "metrics"
	LogHealth    LogCategory = "health"

	// Scanner categories
	LogImporter  LogCategory = "importer"
	LogScanner   LogCategory = "scanner"
	LogFSWatcher LogCategory = "fswatcher"
	LogProcessor LogCategory = "processor"
	LogExtractor LogCategory = "extractor"

	// Plugin categories
	LogPluginExifOpener LogCategory = "opener"
	LogPluginExifCloser LogCategory = "closer"
	LogExif             LogCategory = "exif"
	LogML               LogCategory = "ml"
	LogMetadata         LogCategory = "metadata"
	LogPerceptors       LogCategory = "perceptors"

	// Service categories
	LogAPI        LogCategory = "api"
	LogAuth       LogCategory = "auth"
	LogValidation LogCategory = "validation"
	LogSync       LogCategory = "sync"
	LogEvents     LogCategory = "events"
	LogQueue      LogCategory = "queue"

	// Data categories
	LogFiles      LogCategory = "files"
	LogThumbnails LogCategory = "thumbnails"
	LogAlbums     LogCategory = "albums"
	LogTags       LogCategory = "tags"
	LogSearch     LogCategory = "search"
)

// DefaultCategories повертає список категорій, які увімкнені за замовчуванням
func DefaultCategories() []LogCategory {
	return []LogCategory{
		LogCore,
		LogConfig,
		LogPlugins,
		LogDB,
		LogHTTP,
		LogScanner,
		LogProcessor,
		LogExif,
		LogML,
		LogAPI,
	}
}

// String реалізує інтерфейс Stringer для LogCategory
func (c LogCategory) String() string {
	return string(c)
}
