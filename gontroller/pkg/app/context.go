package app

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogLevel zapcore.Level

var appConfig Config

const (
	DebugLevel  LogLevel = LogLevel(zapcore.DebugLevel)
	InfoLevel   LogLevel = LogLevel(zapcore.InfoLevel)
	WarnLevel   LogLevel = LogLevel(zapcore.WarnLevel)
	ErrorLevel  LogLevel = LogLevel(zapcore.ErrorLevel)
	DPanicLevel LogLevel = LogLevel(zapcore.DPanicLevel)
	PanicLevel  LogLevel = LogLevel(zapcore.PanicLevel)
	FatalLevel  LogLevel = LogLevel(zapcore.FatalLevel)
)

type appContext struct {
	config   *Config
	logger   *zap.Logger
	logLevel LogLevel
}

type AppContext interface {
	Config() *Config
	Logger(category string) *zap.Logger
	SetLogLevel(level LogLevel)
}

func NewAppContext() *appContext {
	//logger, _ := zap.NewProduction()
	logger, _ := zap.NewDevelopment()
	initConfig(&appConfig, logger.Named("App.Config"))

	return &appContext{
		config:   &appConfig,
		logger:   logger,
		logLevel: InfoLevel,
	}
}

func (a *appContext) Config() *Config {
	return a.config
}

func (a *appContext) Logger(category string) *zap.Logger {
	return a.logger.Named(category)
}

func (a *appContext) SetLogLevel(level LogLevel) {
	a.logLevel = level
	zapLevel := zapcore.Level(level)

	a.logger = a.logger.WithOptions(zap.IncreaseLevel(zapLevel))
}
