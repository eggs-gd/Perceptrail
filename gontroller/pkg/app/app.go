package app

import (
	"log"
	"os"
	"sync"
)

type appContext struct {
	config   *Config
	logger   *log.Logger
	wg       sync.WaitGroup
	services []Service
}

type AppContext interface {
	Config() *Config

	AddWg()
	DoneWg()
	AddService(service Service)

	StartApp()
	StopApp()
}

type Service interface {
	Start()
	Stop()
}

var appContextInst appContext

func NewAppContext() *appContext {
	if appContextInst.config == nil {
		appContextInst = appContext{
			config: &appConfig,
			logger: log.New(os.Stdout, "app: ", log.LstdFlags),
		}
	}

	return &appContextInst
}

func (a *appContext) Config() *Config {
	return a.config
}

func (a *appContext) AddWg() {
	a.wg.Add(1)
}

func (a *appContext) DoneWg() {
	a.wg.Done()
}

func (a *appContext) AddService(service Service) {
	a.services = append(a.services, service)
}

func (a *appContext) StartApp() {
	for _, s := range a.services {
		s.Start()
	}
}

func (a *appContext) StopApp() {
	for _, s := range a.services {
		s.Stop()
	}
}
