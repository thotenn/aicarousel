package providers

import "github.com/thotenn/aicarousel-go/internal/providers/provparams"

// Params is a type alias for provparams.Params so callers can use providers.Params
// without importing the leaf provparams package directly.
type Params = provparams.Params

// DefaultParams re-exports provparams.DefaultParams for convenience.
var DefaultParams = provparams.DefaultParams

// Re-export default constants so callers can reference providers.DefaultMaxTokens etc.
const (
	DefaultMaxTokens   = provparams.DefaultMaxTokens
	DefaultTemperature = provparams.DefaultTemperature
	DefaultTopP        = provparams.DefaultTopP
)

// ProviderInfo contains static metadata for a provider.
type ProviderInfo struct {
	Key    string // e.g. "cerebras"
	Name   string // human-readable, e.g. "Cerebras"
	EnvKey string // environment variable that gates this provider
}

// Meta holds static metadata for all known providers.
var Meta = map[string]ProviderInfo{
	"cerebras":   {Key: "cerebras", Name: "Cerebras", EnvKey: "CEREBRAS_API_KEY"},
	"groq":       {Key: "groq", Name: "Groq", EnvKey: "GROQ_API_KEY"},
	"openrouter": {Key: "openrouter", Name: "OpenRouter", EnvKey: "OPENROUTER_API_KEY"},
	"gemini":     {Key: "gemini", Name: "Gemini", EnvKey: "GEMINI_API_KEY"},
	"ollama":     {Key: "ollama", Name: "Ollama", EnvKey: "OLLAMA_ENABLED"},
	"zai":        {Key: "zai", Name: "Z.ai", EnvKey: "ZAI_API_KEY"},
}
