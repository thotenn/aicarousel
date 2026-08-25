package config

// Version is the application version injected at build time via:
//
//	go build -ldflags "-X github.com/thotenn/aicarousel/internal/config.Version=v1.0.0"
//
// Defaults to "dev" when not set.
var Version = "dev"
