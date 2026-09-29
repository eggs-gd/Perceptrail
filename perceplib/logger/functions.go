package logger

import (
	"math/rand"
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
	globalMu sync.RWMutex
	r        = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// Helper functions for creating Fields
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Any(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}

func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

func Level(level LogLevel) LogLevel {
	return LogLevel(zapcore.Level(level))
}

func IncreaseLevel(level LogLevel) Option {
	return Option{zapOption: zap.IncreaseLevel(zapcore.Level(level))}
}

func getColorForService(service string) string {
	globalMu.Lock()
	defer globalMu.Unlock()

	if color, exists := serviceColors[service]; exists {
		return color
	}

	color := colors[r.Intn(len(colors))]
	serviceColors[service] = color
	return color
}
