package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var AppConfig Config

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

func init() {
	configPath := "config.yml"
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		return
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&AppConfig)
	if err != nil {
		fmt.Printf("Error parsing config file: %v\n", err)
		return
	}

	fmt.Println("Configuration loaded successfully")
}
