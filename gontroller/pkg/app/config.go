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
		// CORS: origins allowed to call the API (the client in dev runs on another
		// port). Default: any
		AllowedOrigins []string `yaml:"allowed_origins"`
	} `yaml:"server"`
	// Behind GORM, so the driver can change later; only sqlite for now
	Database struct {
		Driver string `yaml:"driver"`
		// sqlite: the database file, relative to DataDir; server drivers: database name
		Name string `yaml:"name"`
		// Server drivers only; sqlite ignores them
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Token    string `yaml:"token"`
	} `yaml:"database"`
}

const (
	defaultPort     = 1323
	defaultDBDriver = "sqlite"
	defaultDBName   = "media_library.db"
)

// DatabasePath is the SQLite database file (Database.Name inside DataDir).
func (c *Config) DatabasePath() string {
	if filepath.IsAbs(c.Database.Name) {
		return c.Database.Name
	}
	return filepath.Join(c.DataDir, c.Database.Name)
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
	if config.Database.Driver != defaultDBDriver {
		logger.Fatal("Unsupported database driver (only sqlite for now)", l.String("driver", config.Database.Driver))
	}
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
	if c.Database.Driver == "" {
		c.Database.Driver = defaultDBDriver
	}
	if c.Database.Name == "" {
		c.Database.Name = defaultDBName
	}
	if len(c.Server.AllowedOrigins) == 0 {
		c.Server.AllowedOrigins = []string{"*"}
	}
	// A bare command name is looked up in PATH; only paths are resolved
	if filepath.Base(c.Exiftool) != c.Exiftool {
		c.Exiftool = abs(c.Exiftool)
	}
}
