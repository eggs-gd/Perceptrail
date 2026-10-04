// Package config: the server's config file (config.yml), read once at start. A leaf:
// it imports no module of ours. A module declares the getters it reads as its own
// Config interface and is given the whole *Config. Relative paths in the file are
// resolved against its directory, so the result does not depend on the working
// directory: `make run` and `./.build/gontroller --config …` use the same database,
// caches and plugins.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	l "github.com/eggs-gd/perceplib/logger"

	"gopkg.in/yaml.v3"
)

// Config: the config as read, defaults filled, paths absolute
type Config struct {
	path string
	file file
}

// file: the YAML document (see config.example.yml)
type file struct {
	// debug (development: SQL and every request logged, debug marks in the
	// gallery) or release (the default)
	Mode string `yaml:"mode"`
	// Library root: the photos to import
	Path string `yaml:"path"`
	// Plugin files (.so)
	Plugins []string `yaml:"plugins"`
	// Per perceptor (built in and plugins, by name): run it, give it to the client
	Perceptors Perceptors `yaml:"perceptors"`
	// Per provider (a library read through its own means — apple): in the import
	// chain or not; a provider not enabled leaves its files to a plain folder's
	Providers Providers `yaml:"providers"`
	// Runtime data: database, caches. Default: the config file's directory
	DataDir string `yaml:"data_dir"`
	// ExifTool executable. Default: "exiftool" from PATH
	Exiftool string `yaml:"exiftool"`
	// Pause between the end of one pass of the import and the next ("1m", "30s").
	// Default: 1 minute
	Rescan time.Duration `yaml:"rescan"`
	// HTTP: listen address, CORS
	Server Server `yaml:"server"`
	// Behind GORM: sqlite, postgres (not implemented yet)
	Database Database `yaml:"database"`
}

// Modes (Mode)
const (
	ModeDebug   = "debug"
	ModeRelease = "release"
)

const (
	defaultRescan   = time.Minute
	defaultDBName   = "media_library.db"
	defaultExiftool = "exiftool"
)

var configFlag = flag.String("config", "", "path to config.yml (default: $GONTROLLER_CONFIG, then ./config.yml)")

// Load reads the config: --config, then $GONTROLLER_CONFIG, then ./config.yml
func Load() (*Config, error) {
	return Read(configPath())
}

// Read reads the config file at path: fills the defaults, checks the values,
// creates the data directory
func Read(path string) (*Config, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("config path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{path: path}
	if err := yaml.Unmarshal(data, &c.file); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.file.resolve(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := os.MkdirAll(c.file.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("data_dir %s: %w", c.file.DataDir, err)
	}
	return c, nil
}

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

// resolve fills the defaults, makes every relative path absolute against base (the
// config's directory) and checks the values
func (f *file) resolve(base string) error {
	abs := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}

	if f.Mode != ModeDebug {
		f.Mode = ModeRelease // an unknown mode is not a debug one
	}
	f.Path = abs(f.Path)
	if f.DataDir == "" {
		f.DataDir = base
	}
	f.DataDir = abs(f.DataDir)
	for i, p := range f.Plugins {
		f.Plugins[i] = abs(p)
	}
	if f.Rescan <= 0 {
		f.Rescan = defaultRescan
	}
	if f.Exiftool == "" {
		f.Exiftool = defaultExiftool
	}
	// A bare command name is looked up in PATH; only paths are resolved
	if filepath.Base(f.Exiftool) != f.Exiftool {
		f.Exiftool = abs(f.Exiftool)
	}
	if f.Database.Driver == "" {
		f.Database.Driver = DriverSQLite
	}
	if f.Database.Driver == DriverSQLite {
		// sqlite: Name is a file inside DataDir
		if f.Database.Name == "" {
			f.Database.Name = defaultDBName
		}
		if !filepath.IsAbs(f.Database.Name) {
			f.Database.Name = filepath.Join(f.DataDir, f.Database.Name)
		}
	}
	return f.Server.resolve()
}

// File: the config file read
func (c *Config) File() string { return c.path }

// Mode: debug or release
func (c *Config) Mode() string { return c.file.Mode }

// Debug: development — SQL and every request logged, debug marks in the gallery
func (c *Config) Debug() bool { return c.file.Mode == ModeDebug }

// LogLevel: debug logs everything; release from Info up
func (c *Config) LogLevel() l.LogLevel {
	if c.Debug() {
		return l.DebugLevel
	}
	return l.InfoLevel
}

// LibraryRoot: the photos to import
func (c *Config) LibraryRoot() string { return c.file.Path }

// DataDir: runtime data — the database, the perceptors' storages, the caches
func (c *Config) DataDir() string { return c.file.DataDir }

// CacheDir: generated files (previews, …) inside DataDir
func (c *Config) CacheDir() string { return filepath.Join(c.file.DataDir, "cache") }

// Plugins: the external perceptors' files (.so)
func (c *Config) Plugins() []string { return c.file.Plugins }

// Perceptors: which perceptors run, which the client is given
func (c *Config) Perceptors() Perceptors { return c.file.Perceptors }

// Providers: which libraries take their files in the import
func (c *Config) Providers() Providers { return c.file.Providers }

// Exiftool: the executable (a bare name is looked up in PATH)
func (c *Config) Exiftool() string { return c.file.Exiftool }

// Rescan: the pause between the end of one pass of the import and the next
func (c *Config) Rescan() time.Duration { return c.file.Rescan }

// Server: the HTTP service's address and CORS
func (c *Config) Server() Server { return c.file.Server }

// Database: where the model keeps its data
func (c *Config) Database() Database { return c.file.Database }
