package gpt

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"gopkg.in/yaml.v3"
)

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

type AppContext struct {
	config   *Config
	logger   *log.Logger
	wg       sync.WaitGroup
	services []Service
}

type Service interface {
	Start()
	Stop()
}

func loadConfig() (*Config, error) {
	configPath := "config.yml"
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Printf("Error parsing config file: %v\n", err)
		return nil, err
	}

	fmt.Println("Configuration loaded successfully")
	return &config, err
}

func Run() {
	config, err := loadConfig()
	if err != nil {
		return
	}

	ctx := &AppContext{
		config: config,
		logger: log.New(os.Stdout, "app: ", log.LstdFlags),
	}

	// Ініціалізація сервісів
	importerService := NewImporterService(ctx)
	webServerService := NewWebServerService(ctx)
	maintenanceService := NewMaintenanceService(ctx)

	ctx.services = []Service{
		importerService,
		webServerService,
		maintenanceService,
	}

	// Запуск сервісів
	for _, service := range ctx.services {
		ctx.wg.Add(1)
		go func(s Service) {
			defer ctx.wg.Done()
			s.Start()
		}(service)
	}

	// Механізм закриття
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	<-shutdown
	ctx.Stop()    // Виклик закриття для всіх сервісів
	ctx.wg.Wait() // Чекаємо на завершення всіх горутин
}

func (ctx *AppContext) Stop() {
	for _, service := range ctx.services {
		service.Stop()
	}
}
