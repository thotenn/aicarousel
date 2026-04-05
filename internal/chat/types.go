package chat

import "context"

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // "user" | "assistant" | "system"
	Content string `json:"content"`
}

// StreamChunk carries one piece of streamed text or a terminal error.
// A non-nil Err means the stream has failed; no further chunks will arrive.
type StreamChunk struct {
	Text string
	Err  error
}

// ActiveProvider describes a provider that is currently enabled and has at
// least one model configured. The Router uses this to drive round-robin and
// intra-provider fallback.
type ActiveProvider struct {
	Key            string   // e.g. "cerebras"
	Name           string   // human-readable, e.g. "Cerebras"
	DefaultModel   string   // primary model to try
	Models         []string // all available models (ordered for fallback)
	EnableFallback bool     // if true, exhaust all models before moving to next provider
	Priority       int      // lower = higher priority (sorted before use)
}

// Provider is implemented by every AI backend adapter.
// The interface is defined here (in the consumer package) to avoid an
// import cycle: internal/providers already imports internal/chat for these types.
type Provider interface {
	Name() string
	Key() string
	Model() string
	Chat(ctx context.Context, msgs []ChatMessage) (<-chan StreamChunk, error)
}
