// Package provparams defines the shared Params type and default values used by
// all AI provider adapters. It lives in its own leaf package to break the
// import cycle that would otherwise arise between internal/providers (registry)
// and internal/providers/* (adapters).
package provparams

// Default generation parameters.
const (
	DefaultMaxTokens   = 4096
	DefaultTemperature = 0.6
	DefaultTopP        = 1.0
)

// Params carries per-request generation parameters passed from the router to
// each adapter's constructor.
type Params struct {
	MaxCompletionTokens int
	Stream              bool
	Temperature         float64
	TopP                float64
	Stop                *string
	Model               string
}

// DefaultParams returns Params populated with standard defaults for model.
func DefaultParams(model string) Params {
	return Params{
		MaxCompletionTokens: DefaultMaxTokens,
		Stream:              true,
		Temperature:         DefaultTemperature,
		TopP:                DefaultTopP,
		Model:               model,
	}
}
