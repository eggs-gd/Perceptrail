# Makefile

.PHONY: run build test

# Version is derived from git history (see ../scripts/version.sh); "dev" if unavailable
VERSION := $(shell ../scripts/version.sh 2>/dev/null || echo dev)
LDFLAGS := -X perceptrail/gontroller/pkg/app.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o ./build/ ./...

test:
	go test ./...

run: build
	go run -ldflags "$(LDFLAGS)" ./...
