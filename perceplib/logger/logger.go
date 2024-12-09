package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(level LogLevel, options ...Option) *Logger {
	zapOptions := make([]zap.Option, len(options))
	for i, opt := range options {
		zapOptions[i] = opt.zapOption
	}

	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.Level(level)),
		Development: true,
		Encoding:    "json",
		OutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:  "message",
			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalColorLevelEncoder,
			TimeKey:     "time",
			EncodeTime:  zapcore.ISO8601TimeEncoder,
		},
	}
	zapLogger, _ := cfg.Build(zapOptions...)
	return &Logger{zapLogger: zapLogger}
}

func (l *Logger) Debug(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.Debug(msg, zapFields...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.Info(msg, zapFields...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.Warn(msg, zapFields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.Error(msg, zapFields...)
}

func (l *Logger) DPanic(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.DPanic(msg, zapFields...)
}

func (l *Logger) Panic(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.Panic(msg, zapFields...)
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	zapFields := convertFields(fields)
	l.zapLogger.Fatal(msg, zapFields...)
}

func (l *Logger) Named(name string) *Logger {
	return &Logger{zapLogger: l.zapLogger.Named(name)}
}

func (l *Logger) With(fields ...Field) *Logger {
	zapFields := convertFields(fields)
	return &Logger{zapLogger: l.zapLogger.With(zapFields...)}
}

func (l *Logger) WithOptions(options ...Option) *Logger {
	zapOptions := make([]zap.Option, len(options))
	for i, opt := range options {
		zapOptions[i] = opt.zapOption
	}
	return &Logger{zapLogger: l.zapLogger.WithOptions(zapOptions...)}
}

func convertFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = zap.Any(field.Key, field.Value)
	}
	return zapFields
}

func convertZapFields(zapFields []zap.Field) []Field {
	fields := make([]Field, len(zapFields))
	for i, zapField := range zapFields {
		fields[i] = Field{
			Key:   zapField.Key,
			Value: zapField.Interface,
		}
	}
	return fields
}
