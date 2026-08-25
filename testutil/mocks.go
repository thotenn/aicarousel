package testutil

import (
	"context"
	"time"

	"github.com/thotenn/aicarousel/internal/chat"
)

// ─── OkProvider ──────────────────────────────────────────────────────────────

// OkProvider streams the given text chunks then closes the channel cleanly.
type OkProvider struct {
	ProviderKey   string
	ProviderName  string
	ProviderModel string
	Chunks        []string
}

func (p *OkProvider) Name() string  { return p.ProviderName }
func (p *OkProvider) Key() string   { return p.ProviderKey }
func (p *OkProvider) Model() string { return p.ProviderModel }

func (p *OkProvider) Chat(ctx context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	ch := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(ch)
		for _, text := range p.Chunks {
			select {
			case ch <- chat.StreamChunk{Text: text}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// ─── EmptyProvider ───────────────────────────────────────────────────────────

// EmptyProvider closes its stream channel immediately without sending any chunk.
type EmptyProvider struct {
	ProviderKey   string
	ProviderModel string
}

func (p *EmptyProvider) Name() string  { return p.ProviderKey }
func (p *EmptyProvider) Key() string   { return p.ProviderKey }
func (p *EmptyProvider) Model() string { return p.ProviderModel }

func (p *EmptyProvider) Chat(_ context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	ch := make(chan chat.StreamChunk, 10)
	close(ch)
	return ch, nil
}

// ─── FailingProvider ─────────────────────────────────────────────────────────

// FailingProvider sends a single StreamChunk with Err set, then closes.
type FailingProvider struct {
	ProviderKey   string
	ProviderModel string
	Err           error
}

func (p *FailingProvider) Name() string  { return p.ProviderKey }
func (p *FailingProvider) Key() string   { return p.ProviderKey }
func (p *FailingProvider) Model() string { return p.ProviderModel }

func (p *FailingProvider) Chat(ctx context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	ch := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(ch)
		select {
		case ch <- chat.StreamChunk{Err: p.Err}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// ─── SlowProvider ────────────────────────────────────────────────────────────

// SlowProvider waits Delay before sending its first chunk.
// Used to trigger first-chunk probe timeouts.
// It also signals Done (a channel the caller can wait on) when its goroutine exits,
// which lets tests verify that cancelled probes clean up promptly.
type SlowProvider struct {
	ProviderKey   string
	ProviderModel string
	Delay         time.Duration
	Chunks        []string
	// Done is closed when the internal goroutine exits. If nil it is ignored.
	Done chan struct{}
}

func (p *SlowProvider) Name() string  { return p.ProviderKey }
func (p *SlowProvider) Key() string   { return p.ProviderKey }
func (p *SlowProvider) Model() string { return p.ProviderModel }

func (p *SlowProvider) Chat(ctx context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	ch := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(ch)
		if p.Done != nil {
			defer close(p.Done)
		}
		select {
		case <-time.After(p.Delay):
		case <-ctx.Done():
			return
		}
		for _, text := range p.Chunks {
			select {
			case ch <- chat.StreamChunk{Text: text}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// ─── SlowDialProvider ────────────────────────────────────────────────────────

// SlowDialProvider blocks inside Chat itself for DialDelay before returning a
// channel — the shape of a real HTTP provider whose POST hangs until the
// upstream sends response headers (Ollama loading a cold model, for instance).
// It is the case a first-chunk-only probe cannot see.
type SlowDialProvider struct {
	ProviderKey   string
	ProviderModel string
	DialDelay     time.Duration
	Chunks        []string
	// Done is closed when Chat returns. If nil it is ignored.
	Done chan struct{}
}

func (p *SlowDialProvider) Name() string  { return p.ProviderKey }
func (p *SlowDialProvider) Key() string   { return p.ProviderKey }
func (p *SlowDialProvider) Model() string { return p.ProviderModel }

func (p *SlowDialProvider) Chat(ctx context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	if p.Done != nil {
		defer close(p.Done)
	}
	select {
	case <-time.After(p.DialDelay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	ch := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(ch)
		for _, text := range p.Chunks {
			select {
			case ch <- chat.StreamChunk{Text: text}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
