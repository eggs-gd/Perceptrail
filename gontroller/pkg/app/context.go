package app

import (
	l "github.com/dukobpa3/perceplib/logger"
	"github.com/dukobpa3/perceplib/logger/decorators"
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

func (a *appContext) Logger(category string) *l.Logger {
	return a.logger.Named(category)
}

func (a *appContext) SetLogLevel(level l.LogLevel) {
	a.logLevel = level
	zapLevel := l.Level(level)

	a.logger = a.logger.WithOptions(l.IncreaseLevel(zapLevel))
}
