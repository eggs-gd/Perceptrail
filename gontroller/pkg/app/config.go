package app

import (
	"flag"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/client"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/plugins/settings"
	"time"

	l "github.com/eggs-gd/perceplib/logger"

	"gopkg.in/yaml.v3"
)

// Config is read from a YAML file (see config.example.yml). Relative paths in it are
// resolved against the directory of the config file, so the result does not depend
// on the working directory: `make run` and `./.build/gontroller --config …` use the
// same database, caches and plugins.
type Config struct {
	// debug (development: SQL and every request logged, debug marks in the
	// gallery) or release (the default)
	Mode string `yaml:"mode"`
	// Library root: the photos to import
	Path string `yaml:"path"`
	// Plugin files (.so)
	Plugins []string `yaml:"plugins"`
	// Per perceptor (core and plugins, by name): run it, give it to the client
	Perceptors settings.Perceptors `yaml:"perceptors"`
	// Per provider (a library read through its own means — apple): in the import
	// chain or not; a provider not enabled leaves its files to a plain folder's
	Providers Providers `yaml:"providers"`
	// Runtime data: database, caches. Default: the config file's directory
	DataDir string `yaml:"data_dir"`
	// ExifTool executable. Default: "exiftool" from PATH
	Exiftool string `yaml:"exiftool"`
	// Pause between the end of one scan's processing and the next scan ("1m",
	// "30s"). Default: 1 minute
	Rescan time.Duration `yaml:"rescan"`
	// HTTP: listen address, CORS
	Server client.ServerConfig `yaml:"server"`
	// Behind GORM: sqlite, postgres (not implemented yet)
	Database model.DBConfig `yaml:"database"`
}

// Modes (Config.Mode)
const (
	ModeDebug   = "debug"
	ModeRelease = "release"
)

func (c *Config) Debug() bool { return c.Mode == ModeDebug }

const (
	defaultDBDriver = model.DriverSQLite
	defaultDBName   = "media_library.db"
)

// CacheDir holds generated files (thumbnails, …) inside DataDir.
func (c *Config) CacheDir() string {
	return filepath.Join(c.DataDir, "cache")
}

var configFlag = flag.String("config", "", "path to config.yml (default: $GONTROLLER_CONFIG, then ./config.yml)")

// configPath: --config, then $GONTROLLER_CONFIG, then ./config.yml.
func configPath() string {
	if !flag.Parsed() {
		flag.Parse()
	}
	if *configFlag != "" {
		return *configFlag
	}
	if env := os.Getenv("GONTROLLER_CONFIG"); env != "" {
		return env
	}
	return "config.yml"
}

func initConfig(config *Config, logger *l.Logger) {
	path, err := filepath.Abs(configPath())
	if err != nil {
		logger.Fatal("Config path", l.Error(err))
	}

	file, err := os.Open(path)
	if err != nil {
		logger.Fatal("Error opening config file", l.String("path", path), l.Error(err))
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(config); err != nil {
		logger.Fatal("Error parsing config file", l.String("path", path), l.Error(err))
	}

	config.resolve(filepath.Dir(path))
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		logger.Fatal("Cannot create data_dir", l.String("data_dir", config.DataDir), l.Error(err))
	}

	logger.Info("Configuration loaded", l.String("config", path), l.String("data_dir", config.DataDir))
}

// resolve makes every relative path absolute against base (the config's directory).
func (c *Config) resolve(base string) {
	abs := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}

	if c.Mode != ModeDebug {
		c.Mode = ModeRelease // an unknown mode is not a debug one
	}
	c.Path = abs(c.Path)
	if c.DataDir == "" {
		c.DataDir = base
	}
	c.DataDir = abs(c.DataDir)
	for i, p := range c.Plugins {
		c.Plugins[i] = abs(p)
	}
	if c.Database.Driver == "" {
		c.Database.Driver = defaultDBDriver
	}
	if c.Database.Driver == model.DriverSQLite {
		// sqlite: Name is a file inside DataDir
		if c.Database.Name == "" {
			c.Database.Name = defaultDBName
		}
		if !filepath.IsAbs(c.Database.Name) {
			c.Database.Name = filepath.Join(c.DataDir, c.Database.Name)
		}
	}
	// A bare command name is looked up in PATH; only paths are resolved
	if filepath.Base(c.Exiftool) != c.Exiftool {
		c.Exiftool = abs(c.Exiftool)
	}
}

// Providers: by name; a provider not listed is enabled
//
//	providers:
//	  apple:
//	    enabled: false   # not in the chain: a Photos library is a plain folder then
type Providers map[string]struct {
	Enabled *bool `yaml:"enabled"`
}

func (p Providers) Enabled(name string) bool {
	c, ok := p[name]
	return !ok || c.Enabled == nil || *c.Enabled
}
