package providers

import (
	"github.com/thotenn/aicarousel/internal/providers/cerebras"
	"github.com/thotenn/aicarousel/internal/providers/gemini"
	"github.com/thotenn/aicarousel/internal/providers/groq"
	"github.com/thotenn/aicarousel/internal/providers/nvidia"
	"github.com/thotenn/aicarousel/internal/providers/ollama"
	"github.com/thotenn/aicarousel/internal/providers/openrouter"
	"github.com/thotenn/aicarousel/internal/providers/zai"
)

// Factory is the constructor signature every adapter must satisfy.
type Factory func(apiKey string, params Params) Provider

// factories maps provider keys to their factory functions.
var factories = map[string]Factory{
	"cerebras":   cerebras.New,
	"groq":       groq.New,
	"openrouter": openrouter.New,
	"gemini":     gemini.New,
	"ollama":     ollama.New,
	"zai":        zai.New,
	"nvidia":     nvidia.New,
}

// New returns the Provider for the given key, constructed with apiKey and
// params. Returns (nil, false) when key is not registered.
func New(key, apiKey string, params Params) (Provider, bool) {
	f, ok := factories[key]
	if !ok {
		return nil, false
	}
	return f(apiKey, params), true
}

// Keys returns the list of all registered provider keys.
func Keys() []string {
	ks := make([]string, 0, len(factories))
	for k := range factories {
		ks = append(ks, k)
	}
	return ks
}
