package app

import (
	"fmt"
	"log"
	"os"
)

type appContext struct {
	config *Config
	logger *log.Logger
}

type AppContext interface {
	Config() *Config
	Logger(category string) *log.Logger
}

func NewAppContext() *appContext {
	return &appContext{
		config: &appConfig,
		logger: log.New(os.Stdout, "app: ", log.LstdFlags),
	}
}

func (a *appContext) Config() *Config {
	return a.config
}

func (a *appContext) Logger(category string) *log.Logger {
	name := fmt.Sprintf("%v/%v:", a.logger.Prefix(), category)
	return log.New(os.Stdout, name, log.LstdFlags)
}
