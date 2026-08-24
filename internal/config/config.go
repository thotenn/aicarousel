package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string

	// Database
	DBPath string

	// Provider API keys
	GroqAPIKey       string
	CerebrasAPIKey   string
	OpenRouterAPIKey string
	GeminiAPIKey     string
	ZaiAPIKey        string
	ZaiBaseURL       string
	NvidiaAPIKey     string

	// Ollama (local LLM — no API key, gated by OLLAMA_ENABLED=true)
	OllamaEnabled bool
	OllamaBaseURL string
	// OllamaNumCtx is the context window in tokens. Ollama's own default is
	// 4096 (2048 on older installs) and it truncates from the front, which is
	// where the system prompt sits.
	OllamaNumCtx int

	// ModelsWithoutSystemRole lists model name fragments whose chat template has
	// no dedicated "system" role. A nil slice means the variable was never set
	// (defaults apply); an empty non-nil slice disables the adaptation.
	ModelsWithoutSystemRole []string

	// Models config override (JSON string; takes precedence over models.json)
	ModelsConfigJSON string

	// Observability
	LogFormat string // "text" | "json"
	LogLevel  string // "debug" | "info" | "warn" | "error"

	// Streaming
	FirstChunkTimeoutMs int // default 3000
}

// Cfg is the process-wide configuration instance, populated by Load.
var Cfg Config

// Load reads the given .env file (if it exists) without overwriting environment
// variables that are already set (system environment wins), then populates Cfg.
// Pass an empty envPath to use the default ".env" in the working directory.
func Load(envPath string) {
	if envPath == "" {
		envPath = ".env"
	}
	// godotenv.Load does NOT overwrite existing env vars — system env takes precedence.
	if err := godotenv.Load(envPath); err != nil && !os.IsNotExist(err) {
		// Ignore "file not found"; surface unexpected errors to stderr without slog
		// because logging may not be configured yet at the time Load is called.
		_ = err // callers can check for presence themselves; non-fatal
	}

	Cfg = Config{
		Port:             getEnv("PORT", "7123"),
		DBPath:           getEnv("DB_PATH", filepath.Join("data", "aicarousel.db")),
		GroqAPIKey:       os.Getenv("GROQ_API_KEY"),
		CerebrasAPIKey:   os.Getenv("CEREBRAS_API_KEY"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		ZaiAPIKey:        os.Getenv("ZAI_API_KEY"),
		ZaiBaseURL:       getEnv("ZAI_BASE_URL", "https://api.z.ai/api/anthropic"),
		NvidiaAPIKey:     os.Getenv("NVIDIA_API_KEY"),
		OllamaEnabled:    strings.EqualFold(getEnv("OLLAMA_ENABLED", "false"), "true"),
		OllamaBaseURL:    getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaNumCtx:     getEnvInt("OLLAMA_NUM_CTX", 8192),

		ModelsWithoutSystemRole: getEnvList("MODELS_WITHOUT_SYSTEM_ROLE"),
		ModelsConfigJSON:        os.Getenv("MODELS_CONFIG"),
		LogFormat:               getEnv("LOG_FORMAT", "text"),
		LogLevel:                getEnv("LOG_LEVEL", "info"),
		FirstChunkTimeoutMs:     getEnvInt("FIRST_CHUNK_TIMEOUT_MS", 3000),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvList parses a comma-separated env var into a lowercased, trimmed slice.
// It returns nil when the variable is unset (so callers can fall back to their
// own defaults) and an empty non-nil slice when it is set but empty.
func getEnvList(key string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}

	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if v := strings.ToLower(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
