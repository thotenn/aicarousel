package chat

import (
	"context"
	"errors"
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
	rr               roundRobin
	timeout          time.Duration
	providerTimeouts map[string]time.Duration
	listActive       func(ctx context.Context) ([]ActiveProvider, error)
	buildProvider    func(key, model string, opts Options) (Provider, error)
}

// Option customises a Router at construction time.
type Option func(*Router)

// WithProviderTimeouts overrides the first-chunk timeout per provider key.
// A local backend (Ollama) needs a far longer budget than a hosted API: it may
// have to load the model into RAM and evaluate the whole prompt before the
// first token appears. Keys not present here use the default timeout.
func WithProviderTimeouts(m map[string]time.Duration) Option {
	return func(r *Router) {
		if len(m) == 0 {
			return
		}
		r.providerTimeouts = make(map[string]time.Duration, len(m))
		for k, v := range m {
			if v > 0 {
				r.providerTimeouts[k] = v
			}
		}
	}
}

// New creates a Router.
//   - timeout: how long to wait for a provider's first chunk, counted from
//     before the request is dialled (probe timeout).
//   - listActive: returns the priority-ordered list of currently enabled providers.
//   - buildProvider: constructs a Provider instance bound to the given key + model,
//     with the caller's per-request options applied over the provider defaults.
func New(
	timeout time.Duration,
	listActive func(ctx context.Context) ([]ActiveProvider, error),
	buildProvider func(key, model string, opts Options) (Provider, error),
	opts ...Option,
) *Router {
	r := &Router{
		timeout:       timeout,
		listActive:    listActive,
		buildProvider: buildProvider,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// timeoutFor returns the first-chunk budget for a provider key.
func (r *Router) timeoutFor(key string) time.Duration {
	if d, ok := r.providerTimeouts[key]; ok && d > 0 {
		return d
	}
	return r.timeout
}

// Handle routes msgs to the next available provider using round-robin selection
// with intra-provider model fallback. It returns a buffered stream channel on
// success; the channel is closed when the stream ends. Errors are only returned
// when no provider could start streaming.
func (r *Router) Handle(ctx context.Context, msgs []ChatMessage, opts Options) (<-chan StreamChunk, error) {
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
			// The caller is gone (client disconnected or cancelled): every
			// remaining attempt would fail instantly with context.Canceled.
			// Stop here instead of burning the whole carousel on a dead request.
			if ctxErr := ctx.Err(); ctxErr != nil {
				slog.InfoContext(ctx, "request cancelled, aborting fallback",
					"after_attempts", attempt, "err", ctxErr)
				return nil, fmt.Errorf("request cancelled after %d attempt(s): %w", attempt, ctxErr)
			}

			attempt++
			p, buildErr := r.buildProvider(ap.Key, model, opts)
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

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("request cancelled after %d attempt(s): %w", attempt, ctxErr)
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

// dialResult carries the outcome of a p.Chat call made off the probe path.
type dialResult struct {
	ch  <-chan StreamChunk
	err error
}

// tryProvider starts p.Chat under a cancel-only context (no timeout — we do
// not want to kill long-running streams after the probe window), but it waits
// for BOTH the dial and the first chunk under a single probe deadline. The dial
// has to be inside the budget: p.Chat blocks until the upstream returns response
// headers, and a local backend loading a model can sit there for a minute — long
// enough for the caller to give up and take the whole carousel down with it.
// On any failure the provider goroutine is cancelled before returning.
func (r *Router) tryProvider(reqCtx context.Context, p Provider, msgs []ChatMessage) (<-chan StreamChunk, error) {
	// providerCtx: drives the actual provider goroutine — no timeout.
	providerCtx, providerCancel := context.WithCancel(reqCtx)

	// probeCtx: the whole time-to-first-chunk budget, dial included.
	probeCtx, probeCancel := context.WithTimeout(reqCtx, r.timeoutFor(p.Key()))
	defer probeCancel()

	// Adapt here rather than in Handle: the target model is only known once a
	// provider has been picked, and fallback may land on a different one.
	adapted := AdaptMessagesForModel(msgs, p.Model())

	dialed := make(chan dialResult, 1)
	go func() {
		ch, err := p.Chat(providerCtx, adapted)
		dialed <- dialResult{ch: ch, err: err}
	}()

	var ch <-chan StreamChunk
	select {
	case d := <-dialed:
		if d.err != nil {
			providerCancel()
			return nil, fmt.Errorf("%s/%s: %w", p.Key(), p.Model(), d.err)
		}
		ch = d.ch

	case <-probeCtx.Done():
		// Cancel the provider goroutine so the in-flight request is aborted and
		// its FDs released; drain whatever the dial produces afterwards.
		providerCancel()
		go drainDial(dialed)
		return nil, r.probeErr(reqCtx, p, "dial")
	}

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
		go drainChan(ch)
		return nil, r.probeErr(reqCtx, p, "first-chunk")
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

// probeErr distinguishes "the caller went away" from "this provider is too slow",
// so the log says which one actually happened.
func (r *Router) probeErr(reqCtx context.Context, p Provider, phase string) error {
	if err := reqCtx.Err(); err != nil {
		return fmt.Errorf("%s/%s: request cancelled: %w", p.Key(), p.Model(), err)
	}
	return fmt.Errorf("%s/%s: %s probe timeout after %s", p.Key(), p.Model(), phase, r.timeoutFor(p.Key()))
}

// drainDial consumes a late dial result and releases whatever it carries.
func drainDial(dialed <-chan dialResult) {
	d := <-dialed
	if d.err == nil && d.ch != nil {
		drainChan(d.ch)
	}
}

// drainChan empties a provider channel so its goroutine can finish and close
// the underlying response body. The provider context is already cancelled by
// the time this runs.
func drainChan(ch <-chan StreamChunk) {
	for range ch { //nolint:revive // draining, values intentionally discarded
	}
}

// IsCancelled reports whether err came from the caller going away rather than
// from a provider failure.
func IsCancelled(err error) bool {
	return errors.Is(err, context.Canceled)
}
