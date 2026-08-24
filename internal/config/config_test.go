package config

import (
	"os"
	"path/filepath"
	"testing"
)

// saveCfg saves the current Cfg and restores it after the test.
func saveCfg(t *testing.T) {
	t.Helper()
	prev := Cfg
	t.Cleanup(func() { Cfg = prev })
}

// noEnvFile returns a path that does not exist, so Load skips file loading.
func noEnvFile(t *testing.T) string {
	return filepath.Join(t.TempDir(), "nonexistent.env")
}

// unsetenv removes key from the environment for the duration of the test and
// restores the original value (or absence) on cleanup.
// Unlike t.Setenv("KEY",""), this makes os.LookupEnv return (_, false).
func unsetenv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old) //nolint:errcheck
		} else {
			os.Unsetenv(key) //nolint:errcheck
		}
	})
}

func TestLoad_defaults(t *testing.T) {
	saveCfg(t)

	// Ensure none of the env vars are set.
	for _, k := range []string{
		"PORT", "DB_PATH", "ZAI_BASE_URL",
		"OLLAMA_ENABLED", "OLLAMA_BASE_URL",
		"LOG_FORMAT", "LOG_LEVEL", "FIRST_CHUNK_TIMEOUT_MS",
	} {
		t.Setenv(k, "")
	}

	Load(noEnvFile(t))

	if Cfg.Port != "7123" {
		t.Errorf("Port: got %q, want %q", Cfg.Port, "7123")
	}
	if Cfg.ZaiBaseURL != "https://api.z.ai/api/anthropic" {
		t.Errorf("ZaiBaseURL: got %q", Cfg.ZaiBaseURL)
	}
	if Cfg.OllamaEnabled {
		t.Error("OllamaEnabled: expected false by default")
	}
	if Cfg.OllamaBaseURL != "http://localhost:11434" {
		t.Errorf("OllamaBaseURL: got %q", Cfg.OllamaBaseURL)
	}
	if Cfg.LogFormat != "text" {
		t.Errorf("LogFormat: got %q, want %q", Cfg.LogFormat, "text")
	}
	if Cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", Cfg.LogLevel, "info")
	}
	if Cfg.FirstChunkTimeoutMs != 3000 {
		t.Errorf("FirstChunkTimeoutMs: got %d, want 3000", Cfg.FirstChunkTimeoutMs)
	}
}

func TestLoad_envOverridesDefaults(t *testing.T) {
	saveCfg(t)

	t.Setenv("PORT", "9000")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("ZAI_BASE_URL", "https://custom.ai/api/anthropic")
	t.Setenv("OLLAMA_ENABLED", "true")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("FIRST_CHUNK_TIMEOUT_MS", "5000")

	Load(noEnvFile(t))

	if Cfg.Port != "9000" {
		t.Errorf("Port: got %q, want %q", Cfg.Port, "9000")
	}
	if Cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath: got %q", Cfg.DBPath)
	}
	if Cfg.ZaiBaseURL != "https://custom.ai/api/anthropic" {
		t.Errorf("ZaiBaseURL: got %q", Cfg.ZaiBaseURL)
	}
	if !Cfg.OllamaEnabled {
		t.Error("OllamaEnabled: expected true")
	}
	if Cfg.OllamaBaseURL != "http://ollama:11434" {
		t.Errorf("OllamaBaseURL: got %q", Cfg.OllamaBaseURL)
	}
	if Cfg.LogFormat != "json" {
		t.Errorf("LogFormat: got %q", Cfg.LogFormat)
	}
	if Cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q", Cfg.LogLevel)
	}
	if Cfg.FirstChunkTimeoutMs != 5000 {
		t.Errorf("FirstChunkTimeoutMs: got %d", Cfg.FirstChunkTimeoutMs)
	}
}

func TestLoad_ollamaEnabledCaseInsensitive(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"yes", false}, // only "true" (case-insensitive) is accepted
		{"1", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			saveCfg(t)
			t.Setenv("OLLAMA_ENABLED", tc.val)
			Load(noEnvFile(t))
			if Cfg.OllamaEnabled != tc.want {
				t.Errorf("OLLAMA_ENABLED=%q: got %v, want %v", tc.val, Cfg.OllamaEnabled, tc.want)
			}
		})
	}
}

func TestLoad_firstChunkTimeoutInvalid(t *testing.T) {
	saveCfg(t)
	t.Setenv("FIRST_CHUNK_TIMEOUT_MS", "not-a-number")
	Load(noEnvFile(t))
	if Cfg.FirstChunkTimeoutMs != 3000 {
		t.Errorf("expected default 3000 on invalid int, got %d", Cfg.FirstChunkTimeoutMs)
	}
}

func TestLoad_apiKeys(t *testing.T) {
	saveCfg(t)
	t.Setenv("GROQ_API_KEY", "groq-key")
	t.Setenv("CEREBRAS_API_KEY", "cerebras-key")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("ZAI_API_KEY", "zai-key")
	Load(noEnvFile(t))

	if Cfg.GroqAPIKey != "groq-key" {
		t.Errorf("GroqAPIKey: got %q", Cfg.GroqAPIKey)
	}
	if Cfg.CerebrasAPIKey != "cerebras-key" {
		t.Errorf("CerebrasAPIKey: got %q", Cfg.CerebrasAPIKey)
	}
	if Cfg.OpenRouterAPIKey != "openrouter-key" {
		t.Errorf("OpenRouterAPIKey: got %q", Cfg.OpenRouterAPIKey)
	}
	if Cfg.GeminiAPIKey != "gemini-key" {
		t.Errorf("GeminiAPIKey: got %q", Cfg.GeminiAPIKey)
	}
	if Cfg.ZaiAPIKey != "zai-key" {
		t.Errorf("ZaiAPIKey: got %q", Cfg.ZaiAPIKey)
	}
}

func TestLoad_envFileLoaded(t *testing.T) {
	saveCfg(t)

	// Write a temp .env file.
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := "PORT=8888\nLOG_FORMAT=json\n"
	if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Fully unset PORT and LOG_FORMAT so godotenv can set them from the file.
	// t.Setenv("PORT", "") is NOT enough — godotenv skips keys that already
	// exist in the environment (even empty ones).
	unsetenv(t, "PORT")
	unsetenv(t, "LOG_FORMAT")

	Load(envFile)

	if Cfg.Port != "8888" {
		t.Errorf("Port from file: got %q, want %q", Cfg.Port, "8888")
	}
	if Cfg.LogFormat != "json" {
		t.Errorf("LogFormat from file: got %q", Cfg.LogFormat)
	}
}

func TestLoad_systemEnvWinsOverFile(t *testing.T) {
	saveCfg(t)

	// Write .env with PORT=8888.
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("PORT=8888\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// System env has PORT=9999 — must win over the file.
	t.Setenv("PORT", "9999")

	Load(envFile)

	if Cfg.Port != "9999" {
		t.Errorf("system env should win: got %q, want %q", Cfg.Port, "9999")
	}
}

func TestLoad_modelsConfigJSON(t *testing.T) {
	saveCfg(t)
	payload := `{"cerebras":{"default":"model-x","enableFallback":true,"models":["model-x"]}}`
	t.Setenv("MODELS_CONFIG", payload)
	Load(noEnvFile(t))
	if Cfg.ModelsConfigJSON != payload {
		t.Errorf("ModelsConfigJSON: got %q", Cfg.ModelsConfigJSON)
	}
}

func TestLoad_OllamaNumCtx(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  int
	}{
		{"default when unset", "", false, 8192},
		{"explicit value", "16384", true, 16384},
		{"invalid falls back", "not-a-number", true, 8192},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveCfg(t)
			if tt.set {
				t.Setenv("OLLAMA_NUM_CTX", tt.value)
			} else {
				unsetenv(t, "OLLAMA_NUM_CTX")
			}

			Load(noEnvFile(t))

			if Cfg.OllamaNumCtx != tt.want {
				t.Errorf("OllamaNumCtx = %d, want %d", Cfg.OllamaNumCtx, tt.want)
			}
		})
	}
}

func TestLoad_ModelsWithoutSystemRole(t *testing.T) {
	t.Run("nil when unset", func(t *testing.T) {
		saveCfg(t)
		unsetenv(t, "MODELS_WITHOUT_SYSTEM_ROLE")

		Load(noEnvFile(t))

		if Cfg.ModelsWithoutSystemRole != nil {
			t.Errorf("want nil (so defaults apply), got %#v", Cfg.ModelsWithoutSystemRole)
		}
	})

	t.Run("parsed, trimmed and lowercased", func(t *testing.T) {
		saveCfg(t)
		t.Setenv("MODELS_WITHOUT_SYSTEM_ROLE", " Gemma , PHI3 ")

		Load(noEnvFile(t))

		got := Cfg.ModelsWithoutSystemRole
		if len(got) != 2 || got[0] != "gemma" || got[1] != "phi3" {
			t.Errorf("got %#v, want [gemma phi3]", got)
		}
	})

	t.Run("empty but set disables the adaptation", func(t *testing.T) {
		saveCfg(t)
		t.Setenv("MODELS_WITHOUT_SYSTEM_ROLE", "")

		Load(noEnvFile(t))

		if Cfg.ModelsWithoutSystemRole == nil {
			t.Fatal("want an empty non-nil slice, got nil")
		}
		if len(Cfg.ModelsWithoutSystemRole) != 0 {
			t.Errorf("want empty slice, got %#v", Cfg.ModelsWithoutSystemRole)
		}
	})
}
