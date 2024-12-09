package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	zapLogger *zap.Logger
}

type LogLevel int8

type Field struct {
	Key   string
	Value interface{}
}

type Option struct {
	zapOption zap.Option
}

const (
	DebugLevel  LogLevel = LogLevel(zapcore.DebugLevel)
	InfoLevel   LogLevel = LogLevel(zapcore.InfoLevel)
	WarnLevel   LogLevel = LogLevel(zapcore.WarnLevel)
	ErrorLevel  LogLevel = LogLevel(zapcore.ErrorLevel)
	DPanicLevel LogLevel = LogLevel(zapcore.DPanicLevel)
	PanicLevel  LogLevel = LogLevel(zapcore.PanicLevel)
	FatalLevel  LogLevel = LogLevel(zapcore.FatalLevel)
)

const (
	ColorBlack   = "\033[30m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"

	ColorBrightBlack   = "\033[90m"
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
	ColorBrightCyan    = "\033[96m"
	ColorBrightWhite   = "\033[97m"

	ColorBgBlack   = "\033[40m"
	ColorBgRed     = "\033[41m"
	ColorBgGreen   = "\033[42m"
	ColorBgYellow  = "\033[43m"
	ColorBgBlue    = "\033[44m"
	ColorBgMagenta = "\033[45m"
	ColorBgCyan    = "\033[46m"
	ColorBgWhite   = "\033[47m"

	ColorReset = "\033[0m"
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

func String(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
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

func NewDevelopment() (*Logger, error) {
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{zapLogger: zapLogger}, nil
}

func Level(level LogLevel) LogLevel {
	return LogLevel(zapcore.Level(level))
}

func IncreaseLevel(level LogLevel) Option {
	return Option{zapOption: zap.IncreaseLevel(zapcore.Level(level))}
}

func getColorForService(service string) string {
	switch service {
	case "serviceA":
		return ColorRed
	case "serviceB":
		return ColorGreen
	case "serviceC":
		return ColorYellow
	default:
		return ColorBlue
	}
}

func coloredEncoderConfig(service string) zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	return config
}

func newColoredLogger(service string) *zap.Logger {
	color := getColorForService(service)

	encoderConfig := coloredEncoderConfig(service)

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	logger := zap.New(core).With(zap.String("service", fmt.Sprintf("%s%s%s", color, service, ColorReset)))

	return logger
}
