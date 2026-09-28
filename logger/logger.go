package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type CustomEncoderDecorator interface {
	Decorate(buf *buffer.Buffer, fields []zapcore.Field) *buffer.Buffer
}

type customConsoleEncoder struct {
	zapcore.Encoder
	Decorator CustomEncoderDecorator
}

func (c *customConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf, err := c.Encoder.EncodeEntry(entry, []zapcore.Field{})
	if err != nil {
		return nil, err
	}

	decoratedBuf := c.Decorator.Decorate(buf, fields)

	return decoratedBuf, nil
}
func newCustomConsoleEncoder(decorator CustomEncoderDecorator) *customConsoleEncoder {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("15:04:05.000"))
	}
	encoderConfig.EncodeDuration = func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(ColorCyan + d.String() + ColorReset)
	}
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	encoderConfig.NewReflectedEncoder = func(w io.Writer) zapcore.ReflectedEncoder {
		return &consoleEncoder{w: w}
	}
	return &customConsoleEncoder{zapcore.NewConsoleEncoder(encoderConfig), decorator}
}

func NewLogger(level LogLevel, decorator CustomEncoderDecorator, options ...Option) *Logger {
	core := zapcore.NewCore(
		newCustomConsoleEncoder(decorator),
		//zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(zapcore.Lock(os.Stdout)),
		zapcore.Level(level),
	)

	stackLevels := []zapcore.Level{
		zapcore.ErrorLevel,
		zapcore.DPanicLevel,
		zapcore.PanicLevel,
		zapcore.FatalLevel,
	}

	defaultOptions := []zap.Option{
		zap.AddStacktrace(zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			for _, stackLevel := range stackLevels {
				if lvl >= stackLevel {
					return true
				}
			}
			return false
		})),
	}

	logger := &Logger{
		zapLogger: zap.New(core, append(defaultOptions, convertOptions(options)...)...),
		services:  make(map[string]ServiceConfig),
		mu:        &sync.RWMutex{},
	}
	return logger.Named("App")
}

func (l *Logger) log(level LogLevel, msg string, fields ...zap.Field) {
	if len(l.zapLogger.Name()) > 0 && !l.isServiceEnabled(l.zapLogger.Name()) {
		return
	}

	switch level {
	case DebugLevel:
		l.zapLogger.Debug(msg, fields...)
	case InfoLevel:
		l.zapLogger.Info(msg, fields...)
	case WarnLevel:
		l.zapLogger.Warn(msg, fields...)
	case ErrorLevel:
		l.zapLogger.Error(msg, fields...)
	case DPanicLevel:
		l.zapLogger.DPanic(msg, fields...)
	case PanicLevel:
		l.zapLogger.Panic(msg, fields...)
	case FatalLevel:
		l.zapLogger.Fatal(msg, fields...)
	}
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.log(DebugLevel, msg, fields...)
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.log(InfoLevel, msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.log(WarnLevel, msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.log(ErrorLevel, msg, fields...)
}

func (l *Logger) DPanic(msg string, fields ...zap.Field) {
	l.log(DPanicLevel, msg, fields...)
}

func (l *Logger) Panic(msg string, fields ...zap.Field) {
	l.log(PanicLevel, msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.log(FatalLevel, msg, fields...)
}

func (l *Logger) Named(name string) *Logger {
	color := getColorForService(name)
	if _, exists := l.services[name]; !exists {
		l.RegisterService(name, color)
	}

	namedLogger := l.zapLogger.Named(fmt.Sprintf("%s%s%s", color, name, ColorReset))
	return &Logger{
		zapLogger: namedLogger,
		services:  l.services,
		mu:        l.mu,
	}
}

func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{
		zapLogger: l.zapLogger.With(fields...),
		services:  l.services,
		mu:        l.mu,
	}
}

func (l *Logger) WithOptions(options ...Option) *Logger {
	zapOptions := convertOptions(options)
	return &Logger{
		zapLogger: l.zapLogger.WithOptions(zapOptions...),
		services:  l.services,
		mu:        l.mu,
	}
}

func convertOptions(options []Option) []zap.Option {
	zapOptions := make([]zap.Option, len(options))
	for i, opt := range options {
		zapOptions[i] = opt.zapOption
	}
	return zapOptions
}

type consoleEncoder struct {
	w io.Writer
}

func (e *consoleEncoder) Encode(v interface{}) error {
	_, err := fmt.Fprintf(e.w, "%+v", v)
	return err
}
