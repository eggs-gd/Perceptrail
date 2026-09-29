// gontroller/pkg/plugins/plugins_test.go
package plugins

import (
	"perceptrail/gontroller/pkg/app"
	"testing"

	l "github.com/dukobpa3/perceplib/logger"
	"github.com/dukobpa3/perceplib/logger/decorators"
)

type mockAppContext struct{}

func (m *mockAppContext) Config() *app.Config {
	return &app.Config{
		Path:    "/test/path",
		Plugins: []string{"test_plugin.so"},
	}
}

func (m *mockAppContext) Logger(category string) *l.Logger {
	logger := l.NewLogger(l.DebugLevel, &decorators.GontrollerDecorator{})
	return logger
}

func (m *mockAppContext) SetLogLevel(level l.LogLevel) {}

func TestLoadExternalPlugins(t *testing.T) {

	ctx := &mockAppContext{}

	// Завантажуємо плагіни
	err := Pm.LoadPlugins(ctx)
	if err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Отримуємо список завантажених плагінів
	loadedPlugins := Pm.GetPlugins()

	// Виводимо інформацію про кожен плагін
	for i, p := range loadedPlugins {
		t.Logf("Plugin %d: Name=%s, Type=%T, Provider=%v, Mode=%v",
			i, p.Name(), p, p.DataProvider(), p.ProcessingMode())
	}

	// Перевіряємо, чи є зовнішні плагіни
	coreCount := 2 // date + size
	if len(loadedPlugins) <= coreCount {
		t.Logf("Warning: No external plugins loaded. Total plugins: %d", len(loadedPlugins))
	} else {
		t.Logf("External plugins loaded: %d", len(loadedPlugins)-coreCount)
	}
}
