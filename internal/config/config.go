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
	// OllamaKeepAlive is how long Ollama keeps the model resident in RAM after
	// a request ("30m", "-1" for forever, "0" to unload immediately). Loading a
	// cold model is the bulk of the time-to-first-token on a small box, so a
	// warm model is what keeps Ollama inside the router's probe budget.
	OllamaKeepAlive string
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
	// FirstChunkTimeoutMsByProvider overrides FirstChunkTimeoutMs per provider,
	// read from FIRST_CHUNK_TIMEOUT_MS_<PROVIDER> (e.g.
	// FIRST_CHUNK_TIMEOUT_MS_OLLAMA=30000). Keys are lowercased provider keys.
	FirstChunkTimeoutMsByProvider map[string]int
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
		OllamaKeepAlive:  os.Getenv("OLLAMA_KEEP_ALIVE"),

		ModelsWithoutSystemRole: getEnvList("MODELS_WITHOUT_SYSTEM_ROLE"),
		ModelsConfigJSON:        os.Getenv("MODELS_CONFIG"),
		LogFormat:               getEnv("LOG_FORMAT", "text"),
		LogLevel:                getEnv("LOG_LEVEL", "info"),
		FirstChunkTimeoutMs:     getEnvInt("FIRST_CHUNK_TIMEOUT_MS", 3000),

		FirstChunkTimeoutMsByProvider: firstChunkTimeoutOverrides(),
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

// firstChunkTimeoutPrefix is the env prefix for per-provider probe timeouts.
const firstChunkTimeoutPrefix = "FIRST_CHUNK_TIMEOUT_MS_"

// firstChunkTimeoutOverrides collects every FIRST_CHUNK_TIMEOUT_MS_<PROVIDER>
// variable into a map keyed by the lowercased provider key. Values that are not
// positive integers are ignored, so a typo falls back to the global default
// instead of disabling the probe.
func firstChunkTimeoutOverrides() map[string]int {
	out := map[string]int{}
	for _, kv := range os.Environ() {
		name, val, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, firstChunkTimeoutPrefix) {
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(name, firstChunkTimeoutPrefix))
		if key == "" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || n <= 0 {
			continue
		}
		out[key] = n
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
