package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger = zap.Logger

type LogLevel = zapcore.Level

type Option = zap.Option

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

func NewProduction() (*Logger, error) {
	return zap.NewProduction()
}

func NewDevelopment() (*Logger, error) {
	l, err := zap.NewDevelopment()
	return l, err
}

func Level(level LogLevel) LogLevel {
	return LogLevel(zapcore.Level(level))
}

func IncreaseLevel(level LogLevel) Option {
	return Option(zap.IncreaseLevel(level))
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

func Error(err error) zapcore.Field {
	return zap.Error(err)
}
