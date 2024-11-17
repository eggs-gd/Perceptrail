package app

import (
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

/*
app_name: "MyApp"
server:
  host: "localhost"
  port: 8080
database:
  driver: "postgres"
  username: "user"
  password: "password"
  database: "mydatabase"
allowed_hosts:
  - "localhost"
  - "example.com"
  - "myapp.com"
api_keys:
  service1: "key1"
  service2: "key2"
  service3: "key3"
*/

type Config struct {
	Path    string `yaml:"path"`
	AppName string `yaml:"app_name"`
	Server  struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Driver   string `yaml:"driver"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	} `yaml:"database"`
	AllowedHosts []string          `yaml:"allowed_hosts"`
	APIKeys      map[string]string `yaml:"api_keys"`
}

func initConfig(config *Config, logger *zap.Logger) {

	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file")
	}

	configPath := os.Getenv("GONTROLLER_CONFIG")
	if configPath == "" {
		logger.Fatal("GONTROLLER_CONFIG not set in .env file")
	}

	file, err := os.Open(configPath)
	if err != nil {
		logger.Fatal("Error opening config file", zap.Error(err))
		return
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		logger.Fatal("Error parsing config file", zap.Error(err))
		return
	}

	logger.Info("Configuration loaded successfully")
}
