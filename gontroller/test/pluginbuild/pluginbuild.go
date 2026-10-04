// Package pluginbuild builds the perceptors in perceptors/ as Go plugins for the
// integration tests, the way the server would load them: with the Go that built the
// test binary and its -race setting (plugin.Open refuses a plugin whose runtime or
// packages differ from the host's).
package pluginbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
)

// Supported: Go plugins run here (Linux and macOS), and the test is not -short
// (each build takes seconds); otherwise the test is skipped
func Supported(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Go plugins: Linux and macOS only")
	}
	if testing.Short() {
		t.Skip("builds perceptors")
	}
}

// Perceptor: perceptors/<name> built into dir; its .so path (perceptors:
// the directory, relative to the test's own)
func Perceptor(t *testing.T, perceptors, name, dir string) string {
	t.Helper()
	so := filepath.Join(dir, name+".so")
	args := []string{"build", "-buildmode=plugin", "-o", so}
	if race() {
		args = append(args, "-race")
	}
	// The Go of this test binary, exactly: the go on PATH switches to it
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN="+runtime.Version())
	cmd.Dir = filepath.Join(perceptors, name)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: build: %v\n%s", name, err, out)
	}
	return so
}

// race: the test binary is built with -race (a plugin must be too)
func race() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, s := range info.Settings {
		if s.Key == "-race" {
			return s.Value == "true"
		}
	}
	return false
}
