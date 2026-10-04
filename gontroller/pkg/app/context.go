package app

import (
	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

var appConfig Config

type appContext struct {
	config   *Config
	logger   *l.Logger
	logLevel l.LogLevel
}

type AppContext interface {
	Config() *Config
	Logger(category LogCategory) *l.Logger
	SetLogLevel(level l.LogLevel)
}

func NewAppContext() *appContext {
	logger := l.NewLogger(l.DebugLevel, &decorators.GontrollerDecorator{})
	initConfig(&appConfig, logger.Named(string(LogConfig)))

	return &appContext{
		config:   &appConfig,
		logger:   logger,
		logLevel: l.InfoLevel,
	}
}

func (a *appContext) Config() *Config {
	return a.config
}

func (a *appContext) Logger(category LogCategory) *l.Logger {
	return a.logger.Named(string(category))
}

func (a *appContext) SetLogLevel(level l.LogLevel) {
	a.logLevel = level
	zapLevel := l.Level(level)

	a.logger = a.logger.WithOptions(l.IncreaseLevel(zapLevel))
}
