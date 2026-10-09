package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	l "github.com/eggs-gd/go-zap-decor"
)

type Logger struct {
	Logger *l.Logger
}

var _ logger.Interface = &Logger{}

func (logger *Logger) LogMode(level logger.LogLevel) logger.Interface { return logger }

func (logger *Logger) Info(ctx context.Context, msg string, args ...interface{}) {
	logger.Logger.Info(msg, l.Any("args", args))
}

func (logger *Logger) Warn(ctx context.Context, msg string, args ...interface{}) {
	logger.Logger.Warn(msg, l.Any("args", args))
}

func (logger *Logger) Error(ctx context.Context, msg string, args ...interface{}) {
	logger.Logger.Error(msg, l.Any("args", args))
}

func (logger *Logger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		// First() miss is expected during scan validation ("is this file/item new?")
		logger.Logger.Debug("SQL",
			l.Duration("elapsed", elapsed),
			l.Int64("rows", rows),
			l.String("sql", sql))
		return
	}

	logger.Logger.Error("SQL",
		l.Duration("elapsed", elapsed),
		l.Int64("rows", rows),
		l.String("sql", sql),
		l.Error(err))
}

func newLogger(logger *l.Logger) *Logger {
	return &Logger{Logger: logger}
}
