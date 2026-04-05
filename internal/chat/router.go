package chat

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// roundRobin is a concurrency-safe round-robin index.
type roundRobin struct {
	mu  sync.Mutex
	idx int
}

// Snapshot returns the starting provider index for a new request.
// It is the only read path for the index.
func (r *roundRobin) Snapshot(n int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n == 0 {
		return 0
	}
	return r.idx % n
}

// Advance moves the index to the slot after servedIdx.
// Must be called ONLY after a successful stream — failures do not burn the slot.
func (r *roundRobin) Advance(servedIdx, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n == 0 {
		return
	}
	r.idx = (servedIdx + 1) % n
}

// Router selects providers in round-robin order with optional intra-provider
// model fallback (controlled by ActiveProvider.EnableFallback).
type Router struct {
	rr            roundRobin
	timeout       time.Duration
	listActive    func(ctx context.Context) ([]ActiveProvider, error)
	buildProvider func(key, model string) (Provider, error)
}

// New creates a Router.
//   - timeout: how long to wait for the first chunk from a provider (probe timeout).
//   - listActive: returns the priority-ordered list of currently enabled providers.
//   - buildProvider: constructs a Provider instance bound to the given key + model.
func New(
	timeout time.Duration,
	listActive func(ctx context.Context) ([]ActiveProvider, error),
	buildProvider func(key, model string) (Provider, error),
) *Router {
	return &Router{
		timeout:       timeout,
		listActive:    listActive,
		buildProvider: buildProvider,
	}
}

// Handle routes msgs to the next available provider using round-robin selection
// with intra-provider model fallback. It returns a buffered stream channel on
// success; the channel is closed when the stream ends. Errors are only returned
// when no provider could start streaming.
func (r *Router) Handle(ctx context.Context, msgs []ChatMessage) (<-chan StreamChunk, error) {
	providers, err := r.listActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active providers: %w", err)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("no active providers configured")
	}

	n := len(providers)
	start := r.rr.Snapshot(n)
	attempt := 0

	for i := 0; i < n; i++ {
		idx := (start + i) % n
		ap := providers[idx]

		for _, model := range buildModelList(ap) {
			attempt++
			p, buildErr := r.buildProvider(ap.Key, model)
			if buildErr != nil {
				slog.WarnContext(ctx, "build provider failed",
					"provider", ap.Key, "model", model,
					"attempt", attempt, "err", buildErr)
				continue
			}

			ch, tryErr := r.tryProvider(ctx, p, msgs)
			if tryErr != nil {
				slog.WarnContext(ctx, "provider failed, trying next",
					"provider", ap.Key, "model", model,
					"attempt", attempt, "err", tryErr)
				continue
			}

			// Success — advance round-robin so the next request picks the next provider.
			r.rr.Advance(idx, n)
			slog.InfoContext(ctx, "provider chosen",
				"provider", ap.Key, "model", model, "attempt", attempt)
			return ch, nil
		}
	}

	return nil, fmt.Errorf("all providers exhausted after %d attempt(s)", attempt)
}

// buildModelList returns the ordered list of models to try for a provider.
// If EnableFallback is false, only the default model is included.
// If EnableFallback is true, the default model is first, followed by the
// remaining models in their declared order.
func buildModelList(ap ActiveProvider) []string {
	if !ap.EnableFallback {
		return []string{ap.DefaultModel}
	}
	out := make([]string, 0, len(ap.Models)+1)
	out = append(out, ap.DefaultModel)
	for _, m := range ap.Models {
		if m != ap.DefaultModel {
			out = append(out, m)
		}
	}
	return out
}

// tryProvider starts p.Chat under a cancel-only context (no timeout — we do
// not want to kill long-running streams after the probe window). It then waits
// for the first chunk under a separate timeout context (probeCtx). On success
// it spawns a forwarding goroutine and returns the output channel. On any
// failure the provider goroutine is cancelled before returning.
func (r *Router) tryProvider(reqCtx context.Context, p Provider, msgs []ChatMessage) (<-chan StreamChunk, error) {
	// providerCtx: drives the actual provider goroutine — no timeout.
	providerCtx, providerCancel := context.WithCancel(reqCtx)

	ch, err := p.Chat(providerCtx, msgs)
	if err != nil {
		providerCancel()
		return nil, fmt.Errorf("%s/%s: %w", p.Key(), p.Model(), err)
	}

	// probeCtx: used only for the first-chunk wait.
	probeCtx, probeCancel := context.WithTimeout(reqCtx, r.timeout)
	defer probeCancel()

	var first StreamChunk
	var ok bool
	select {
	case first, ok = <-ch:
		if !ok {
			// Provider closed the channel immediately — empty/aborted stream.
			providerCancel()
			return nil, fmt.Errorf("%s/%s: empty stream", p.Key(), p.Model())
		}
		if first.Err != nil {
			providerCancel()
			return nil, fmt.Errorf("%s/%s: stream error: %w", p.Key(), p.Model(), first.Err)
		}

	case <-probeCtx.Done():
		// Cancel the provider goroutine so it stops and closes any open FDs.
		providerCancel()
		if reqCtx.Err() != nil {
			return nil, fmt.Errorf("%s/%s: request cancelled: %w", p.Key(), p.Model(), reqCtx.Err())
		}
		return nil, fmt.Errorf("%s/%s: first-chunk probe timeout", p.Key(), p.Model())
	}

	// First chunk received — forward all remaining chunks.
	out := make(chan StreamChunk, 10)
	go func() {
		defer providerCancel() // stop provider when forwarding ends
		defer close(out)

		// Emit the already-received first chunk.
		select {
		case out <- first:
		case <-reqCtx.Done():
			return
		}

		// Forward remaining chunks.
		for chunk := range ch {
			select {
			case out <- chunk:
			case <-reqCtx.Done():
				return
			}
		}
	}()

	return out, nil
}
