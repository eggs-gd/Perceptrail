package app

import (
	l "perceptrail/logger"
)

var appConfig Config

type appContext struct {
	config   *Config
	logger   *l.Logger
	logLevel l.LogLevel
}

type AppContext interface {
	Config() *Config
	Logger(category string) *l.Logger
	SetLogLevel(level l.LogLevel)
}

func NewAppContext() *appContext {
	//logger, _ := zap.NewProduction()
	logger, _ := l.NewDevelopment()
	initConfig(&appConfig, logger.Named("App.Config"))

	return &appContext{
		config:   &appConfig,
		logger:   logger,
		logLevel: l.InfoLevel,
	}
}

func (a *appContext) Config() *Config {
	return a.config
}

func (a *appContext) Logger(category string) *l.Logger {
	return a.logger.Named(category)
}

func (a *appContext) SetLogLevel(level l.LogLevel) {
	a.logLevel = level
	zapLevel := l.Level(level)

	a.logger = a.logger.WithOptions(l.IncreaseLevel(zapLevel))
}
