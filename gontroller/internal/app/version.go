package app

// Version is set at build time from scripts/version.sh:
//
//	go build -ldflags "-X perceptrail/gontroller/internal/app.Version=$(../scripts/version.sh)"
var Version = "dev"
