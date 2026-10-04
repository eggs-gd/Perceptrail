package perceptor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"perceptrail/gontroller/pkg/config"
	"perceptrail/gontroller/pkg/perceptor"
	"perceptrail/gontroller/pkg/perceptor/builtin"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// Not written yet: a bare `package main`, no Perceptor symbol
var stubPerceptors = map[string]bool{"ml_faces": true, "ml_objects": true}

// Every perceptor in perceptors/ is built as a plugin and loaded into this process,
// as the server does. plugin.Open refuses a plugin built against other versions of a
// package the host has (it happened: x/sync, testify) — `go build` alone does not
// catch that, only loading does.
//
// go test caches the result and does not see the plugins change (they are outside
// this module): after changing one, run it with -count=1 — CI does.
func TestLoadExternalPlugins(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Go plugins: Linux and macOS only")
	}
	if testing.Short() {
		t.Skip("builds every perceptor")
	}
	// The Go that built this test: a plugin must come from the same toolchain
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	root, err := filepath.Abs("../../../perceptors")
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	var files []string
	for _, d := range dirs {
		name := d.Name()
		if _, err := os.Stat(filepath.Join(root, name, "go.mod")); err != nil || stubPerceptors[name] {
			continue
		}
		so := filepath.Join(out, name+".so")
		cmd := exec.Command(goBin, "build", "-buildmode=plugin", "-o", so)
		cmd.Dir = filepath.Join(root, name)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: build: %v\n%s", name, err, b)
		}
		files = append(files, so)
	}
	if len(files) == 0 {
		t.Fatal("no perceptors found in " + root)
	}

	// Loaded as the server does: the config lists the files
	yml := "plugins:\n"
	for _, f := range files {
		yml += "  - " + f + "\n"
	}
	if err := os.WriteFile(filepath.Join(out, "config.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(filepath.Join(out, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := perceptor.Load(cfg, l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})); err != nil {
		t.Fatal(err)
	}
	external := 0
	for _, p := range perceptor.All() {
		if _, built := p.(builtin.Perceptor); built {
			continue
		}
		external++
		if p.Name() == "" || p.View().Slug == "" {
			t.Errorf("loaded, but no name or view slug: %q %q", p.Name(), p.View().Slug)
		}
	}
	if external != len(files) {
		t.Errorf("loaded %d external perceptors of %d built (a plugin that does not load is only logged)", external, len(files))
	}
}
