package perceptor_test

import (
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/internal/config"
	"perceptrail/gontroller/internal/perceptor"
	"perceptrail/gontroller/internal/perceptor/builtin"
	"perceptrail/gontroller/test/pluginbuild"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// Not written yet: a bare `package main`, no Perceptor symbol
var stubPerceptors = map[string]bool{"ml_faces": true, "ml_objects": true}

// Every perceptor in perceptors/ is built as a plugin and loaded into this process,
// as the server does. plugin.Open refuses a plugin built against other versions of a
// package the host has (it happened: x/sync, testify) — `go build` alone does not
// catch that, only loading does. Under -race the plugins are built with it too.
//
// go test caches the result and does not see the plugins change (they are outside
// this module): after changing one, run it with -count=1 — CI does.
func TestLoadExternalPlugins(t *testing.T) {
	pluginbuild.Supported(t)
	const perceptors = "../../../perceptors"
	dirs, err := os.ReadDir(perceptors)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	var files []string
	for _, d := range dirs {
		name := d.Name()
		if _, err := os.Stat(filepath.Join(perceptors, name, "go.mod")); err != nil || stubPerceptors[name] {
			continue
		}
		files = append(files, pluginbuild.Perceptor(t, perceptors, name, out))
	}
	if len(files) == 0 {
		t.Fatal("no perceptors found in " + perceptors)
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
