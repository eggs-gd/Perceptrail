package logger

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	serviceColors = make(map[string]string)
	colors        = []string{
		ColorRed, ColorGreen, ColorYellow, ColorBlue,
		ColorMagenta, ColorCyan, ColorWhite, ColorBrightBlack,
		ColorBrightRed, ColorBrightGreen, ColorBrightYellow,
		ColorBrightBlue, ColorBrightMagenta, ColorBrightCyan,
		ColorBrightWhite,
	}
	mu sync.Mutex
	r  = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func String(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
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
	mu.Lock()
	defer mu.Unlock()

	if color, exists := serviceColors[service]; exists {
		return color
	}

	color := colors[r.Intn(len(colors))]
	serviceColors[service] = color
	return color
}

func coloredEncoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	return config
}

func newColoredLogger(service string) *zap.Logger {
	color := getColorForService(service)

	encoderConfig := coloredEncoderConfig()

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	logger := zap.New(core).With(zap.String("service", fmt.Sprintf("%s%s%s", color, service, ColorReset)))

	return logger
}
