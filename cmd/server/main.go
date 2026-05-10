package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/internal/config"
	"github.com/thotenn/aicarousel/internal/db"
	"github.com/thotenn/aicarousel/internal/db/apikeys"
	"github.com/thotenn/aicarousel/internal/db/provsettings"
	"github.com/thotenn/aicarousel/internal/httpapi"
	anthropichandler "github.com/thotenn/aicarousel/internal/httpapi/anthropic"
	healthhandler "github.com/thotenn/aicarousel/internal/httpapi/health"
	legacyhandler "github.com/thotenn/aicarousel/internal/httpapi/legacy"
	openaihandler "github.com/thotenn/aicarousel/internal/httpapi/openai"
	"github.com/thotenn/aicarousel/internal/modelsconfig"
	"github.com/thotenn/aicarousel/internal/providers"
	"github.com/thotenn/aicarousel/internal/providers/provparams"
)

var version = "dev"

func main() {
	config.Load("")
	setupLogging()

	// Handle "migrate" subcommand.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate()
		return
	}

	d, err := db.Open(config.Cfg.DBPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer d.Close() //nolint:errcheck

	if err := db.Up(d); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	keysRepo := apikeys.New(d)
	provRepo := provsettings.New(d)

	// Sync provider_settings with the current models config so that providers
	// added after the initial DB creation (e.g. zai added via MODELS_CONFIG)
	// are automatically registered on startup.
	if cfg, err := modelsconfig.Load(); err == nil {
		keys := make([]string, 0, len(cfg))
		for k := range cfg {
			keys = append(keys, k)
		}
		if err := provRepo.Sync(context.Background(), keys); err != nil {
			slog.Warn("provider sync", "err", err)
		}
	}

	timeout := time.Duration(config.Cfg.FirstChunkTimeoutMs) * time.Millisecond
	router := chat.New(timeout, buildListActive(provRepo), buildProviderFn())

	mux := http.NewServeMux()
	healthhandler.Register(mux)
	openaihandler.New(router).Register(mux)
	anthropichandler.New(router).Register(mux)
	legacyhandler.New(router).Register(mux)

	handler := httpapi.New(keysRepo, mux)

	addr := ":" + config.Cfg.Port
	slog.Info("AICarousel starting", "addr", addr, "version", version)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func runMigrate() {
	d, err := db.Open(config.Cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: open db: %v\n", err)
		os.Exit(1)
	}
	defer d.Close() //nolint:errcheck
	if err := db.Up(d); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("migrations applied")
}

// buildListActive returns a listActive function for chat.New. It queries
// enabled providers from DB and cross-references API key availability and
// models config.
func buildListActive(provRepo provsettings.Repo) func(context.Context) ([]chat.ActiveProvider, error) {
	return func(ctx context.Context) ([]chat.ActiveProvider, error) {
		settings, err := provRepo.Enabled(ctx)
		if err != nil {
			return nil, fmt.Errorf("list active: %w", err)
		}

		cfg, err := modelsconfig.Load()
		if err != nil {
			return nil, fmt.Errorf("list active: models config: %w", err)
		}

		var active []chat.ActiveProvider
		for _, s := range settings {
			info, ok := providers.Meta[s.ProviderKey]
			if !ok {
				continue
			}
			// Check provider has a configured API key (or Ollama is enabled).
			if !providerIsAvailable(info) {
				continue
			}
			mc, ok := cfg[s.ProviderKey]
			if !ok || len(mc.Models) == 0 {
				continue
			}
			active = append(active, chat.ActiveProvider{
				Key:            s.ProviderKey,
				Name:           info.Name,
				DefaultModel:   mc.Default,
				Models:         mc.Models,
				EnableFallback: mc.EnableFallback,
				Priority:       s.Priority,
			})
		}
		return active, nil
	}
}

// providerIsAvailable reports whether the provider's required environment
// variable (API key or OLLAMA_ENABLED) is set.
func providerIsAvailable(info providers.ProviderInfo) bool {
	v := os.Getenv(info.EnvKey)
	return v != "" && v != "false" && v != "0"
}

// buildProviderFn returns a buildProvider function for chat.New.
func buildProviderFn() func(key, model string) (chat.Provider, error) {
	return func(key, model string) (chat.Provider, error) {
		apiKey := providerAPIKey(key)
		params := provparams.DefaultParams(model)
		p, ok := providers.New(key, apiKey, params)
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", key)
		}
		return p, nil
	}
}

// providerAPIKey returns the API key for the given provider key, reading from
// the typed config struct.
func providerAPIKey(key string) string {
	switch key {
	case "cerebras":
		return config.Cfg.CerebrasAPIKey
	case "groq":
		return config.Cfg.GroqAPIKey
	case "openrouter":
		return config.Cfg.OpenRouterAPIKey
	case "gemini":
		return config.Cfg.GeminiAPIKey
	case "zai":
		return config.Cfg.ZaiAPIKey
	case "nvidia":
		return config.Cfg.NvidiaAPIKey
	case "ollama":
		return "" // Ollama uses no API key
	default:
		return os.Getenv(key + "_API_KEY")
	}
}

func setupLogging() {
	level := slog.LevelInfo
	switch config.Cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var h slog.Handler
	if config.Cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(h))
}
