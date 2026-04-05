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

	// Ollama (local LLM — no API key, gated by OLLAMA_ENABLED=true)
	OllamaEnabled bool
	OllamaBaseURL string

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
		Port:                getEnv("PORT", "7123"),
		DBPath:              getEnv("DB_PATH", filepath.Join("data", "aicarousel.db")),
		GroqAPIKey:          os.Getenv("GROQ_API_KEY"),
		CerebrasAPIKey:      os.Getenv("CEREBRAS_API_KEY"),
		OpenRouterAPIKey:    os.Getenv("OPENROUTER_API_KEY"),
		GeminiAPIKey:        os.Getenv("GEMINI_API_KEY"),
		ZaiAPIKey:           os.Getenv("ZAI_API_KEY"),
		ZaiBaseURL:          getEnv("ZAI_BASE_URL", "https://api.z.ai/api/anthropic"),
		OllamaEnabled:       strings.EqualFold(getEnv("OLLAMA_ENABLED", "false"), "true"),
		OllamaBaseURL:       getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		ModelsConfigJSON:    os.Getenv("MODELS_CONFIG"),
		LogFormat:           getEnv("LOG_FORMAT", "text"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		FirstChunkTimeoutMs: getEnvInt("FIRST_CHUNK_TIMEOUT_MS", 3000),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
