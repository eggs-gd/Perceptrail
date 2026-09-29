package app

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"

	l "github.com/eggs-gd/perceplib/logger"

	"gopkg.in/yaml.v3"
)

// Config is read from a YAML file (see config.example.yml). Relative paths in it are
// resolved against the directory of the config file, so the result does not depend
// on the working directory: `make run` and `./.build/gontroller --config …` use the
// same database, caches and plugins.
type Config struct {
	// Library root: the photos to import
	Path string `yaml:"path"`
	// Plugin files (.so)
	Plugins []string `yaml:"plugins"`
	// Runtime data: database, caches. Default: the config file's directory
	DataDir string `yaml:"data_dir"`
	// ExifTool executable. Default: "exiftool" from PATH
	Exiftool string `yaml:"exiftool"`
	Server   struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
}

const defaultPort = 1323

// DatabasePath is the SQLite database inside DataDir.
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "media_library.db")
}

// CacheDir holds generated files (thumbnails, …) inside DataDir.
func (c *Config) CacheDir() string {
	return filepath.Join(c.DataDir, "cache")
}

// Addr is the HTTP listen address.
func (c *Config) Addr() string {
	port := c.Server.Port
	if port == 0 {
		port = defaultPort
	}
	return c.Server.Host + ":" + strconv.Itoa(port)
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

	c.Path = abs(c.Path)
	if c.DataDir == "" {
		c.DataDir = base
	}
	c.DataDir = abs(c.DataDir)
	for i, p := range c.Plugins {
		c.Plugins[i] = abs(p)
	}
	// A bare command name is looked up in PATH; only paths are resolved
	if filepath.Base(c.Exiftool) != c.Exiftool {
		c.Exiftool = abs(c.Exiftool)
	}
}
