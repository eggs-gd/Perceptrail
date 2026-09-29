# Makefile

.PHONY: run build plugins test

# Version is derived from git history (see ../scripts/version.sh); "dev" if unavailable
VERSION := $(shell ../scripts/version.sh 2>/dev/null || echo dev)
LDFLAGS := -X perceptrail/gontroller/pkg/app.Version=$(VERSION)

# Build artifacts go to .build/ (binary + plugins/*.so), runtime data to .var/
build:
	go build -ldflags "$(LDFLAGS)" -o .build/gontroller .

plugins:
	$(MAKE) -C .. build-plugins

test:
	go test ./...

# Paths in .var/config.yml are relative to .var/ (see config.example.yml)
run: build plugins
	./.build/gontroller --config .var/config.yml
