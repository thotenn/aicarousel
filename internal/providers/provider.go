// Package providers contains the Provider interface, shared request parameters,
// static metadata, and the factory registry for all AI backend adapters.
package providers

import "github.com/thotenn/aicarousel/internal/chat"

// Provider is the interface every AI backend adapter must implement.
// It is a type alias for chat.Provider so the consumer (internal/chat) and
// the implementations (internal/providers/*) share the exact same type without
// creating an import cycle.
type Provider = chat.Provider
