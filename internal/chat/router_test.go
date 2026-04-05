package chat_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/testutil"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newRouter wires a Router using slice-based provider lookup.
func newRouter(
	timeout time.Duration,
	aps []chat.ActiveProvider,
	registry map[string]chat.Provider, // "key/model" → Provider
) *chat.Router {
	listActive := func(_ context.Context) ([]chat.ActiveProvider, error) {
		return aps, nil
	}
	buildP := func(key, model string) (chat.Provider, error) {
		id := key + "/" + model
		p, ok := registry[id]
		if !ok {
			return nil, fmt.Errorf("no provider for %s", id)
		}
		return p, nil
	}
	return chat.New(timeout, listActive, buildP)
}

// drain collects all text chunks from ch, returning them joined and the first
// error chunk seen (if any).
func drain(ch <-chan chat.StreamChunk) (string, error) {
	var b strings.Builder
	var firstErr error
	for c := range ch {
		if c.Err != nil && firstErr == nil {
			firstErr = c.Err
		}
		b.WriteString(c.Text)
	}
	return b.String(), firstErr
}

// sampleAP returns a minimal ActiveProvider for tests.
func sampleAP(key, model string) chat.ActiveProvider {
	return chat.ActiveProvider{
		Key:          key,
		Name:         key,
		DefaultModel: model,
		Models:       []string{model},
		Priority:     1,
	}
}

// ─── basic success ────────────────────────────────────────────────────────────

func TestRouter_SingleProvider_Success(t *testing.T) {
	ap := sampleAP("p1", "m1")
	p := &testutil.OkProvider{ProviderKey: "p1", ProviderModel: "m1", Chunks: []string{"hello", " world"}}
	r := newRouter(100*time.Millisecond, []chat.ActiveProvider{ap}, map[string]chat.Provider{"p1/m1": p})

	ch, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected Handle error: %v", err)
	}
	got, _ := drain(ch)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

// ─── no providers ─────────────────────────────────────────────────────────────

func TestRouter_NoActiveProviders_ReturnsError(t *testing.T) {
	listActive := func(_ context.Context) ([]chat.ActiveProvider, error) { return nil, nil }
	r := chat.New(100*time.Millisecond, listActive, nil)

	_, err := r.Handle(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for no providers")
	}
}

func TestRouter_ListActiveError_PropagatesError(t *testing.T) {
	sentinel := errors.New("db down")
	listActive := func(_ context.Context) ([]chat.ActiveProvider, error) { return nil, sentinel }
	r := chat.New(100*time.Millisecond, listActive, nil)

	_, err := r.Handle(context.Background(), nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// ─── fallback ─────────────────────────────────────────────────────────────────

func TestRouter_CrossProvider_Fallback(t *testing.T) {
	// Provider 1 fails; provider 2 succeeds.
	aps := []chat.ActiveProvider{
		sampleAP("p1", "m1"),
		sampleAP("p2", "m2"),
	}
	failErr := errors.New("upstream 503")
	registry := map[string]chat.Provider{
		"p1/m1": &testutil.FailingProvider{ProviderKey: "p1", ProviderModel: "m1", Err: failErr},
		"p2/m2": &testutil.OkProvider{ProviderKey: "p2", ProviderModel: "m2", Chunks: []string{"from p2"}},
	}
	r := newRouter(100*time.Millisecond, aps, registry)

	ch, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := drain(ch)
	if got != "from p2" {
		t.Errorf("got %q, want %q", got, "from p2")
	}
}

func TestRouter_IntraProvider_Fallback(t *testing.T) {
	// Provider has two models; default (m1) streams nothing (empty), fallback m2 succeeds.
	ap := chat.ActiveProvider{
		Key:            "p1",
		Name:           "P1",
		DefaultModel:   "m1",
		Models:         []string{"m1", "m2"},
		EnableFallback: true,
	}
	registry := map[string]chat.Provider{
		"p1/m1": &testutil.EmptyProvider{ProviderKey: "p1", ProviderModel: "m1"},
		"p1/m2": &testutil.OkProvider{ProviderKey: "p1", ProviderModel: "m2", Chunks: []string{"m2 text"}},
	}
	r := newRouter(100*time.Millisecond, []chat.ActiveProvider{ap}, registry)

	ch, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := drain(ch)
	if got != "m2 text" {
		t.Errorf("got %q, want %q", got, "m2 text")
	}
}

func TestRouter_FallbackDisabled_SkipsToNextProvider(t *testing.T) {
	// p1 has two models but enableFallback=false; only default is tried, then p2.
	p1 := chat.ActiveProvider{
		Key:            "p1",
		Name:           "P1",
		DefaultModel:   "m1",
		Models:         []string{"m1", "m2"},
		EnableFallback: false, // must NOT try m2
	}
	p2 := sampleAP("p2", "m3")

	m2Called := false
	registry := map[string]chat.Provider{
		"p1/m1": &testutil.EmptyProvider{ProviderKey: "p1", ProviderModel: "m1"},
		"p1/m2": &testutil.OkProvider{
			ProviderKey:   "p1",
			ProviderModel: "m2",
			Chunks:        []string{"should not reach"},
		},
		"p2/m3": &testutil.OkProvider{ProviderKey: "p2", ProviderModel: "m3", Chunks: []string{"p2 ok"}},
	}

	// Wrap m2 to detect if it's called.
	registry["p1/m2"] = &callTrackProvider{
		Provider: registry["p1/m2"],
		called:   &m2Called,
	}

	r := newRouter(100*time.Millisecond, []chat.ActiveProvider{p1, p2}, registry)

	ch, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := drain(ch)

	if m2Called {
		t.Error("p1/m2 must not be called when enableFallback=false")
	}
	if got != "p2 ok" {
		t.Errorf("got %q, want %q", got, "p2 ok")
	}
}

func TestRouter_AllProvidersFail_ReturnsError(t *testing.T) {
	aps := []chat.ActiveProvider{sampleAP("p1", "m1"), sampleAP("p2", "m2")}
	err1 := errors.New("fail1")
	err2 := errors.New("fail2")
	registry := map[string]chat.Provider{
		"p1/m1": &testutil.FailingProvider{ProviderKey: "p1", ProviderModel: "m1", Err: err1},
		"p2/m2": &testutil.FailingProvider{ProviderKey: "p2", ProviderModel: "m2", Err: err2},
	}
	r := newRouter(100*time.Millisecond, aps, registry)

	_, err := r.Handle(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("error should mention exhausted providers, got: %v", err)
	}
}

// ─── round-robin ─────────────────────────────────────────────────────────────

func TestRouter_RoundRobin_AdvancesOnSuccess(t *testing.T) {
	p1 := &callTrackProvider{
		Provider: &testutil.OkProvider{ProviderKey: "p1", ProviderModel: "m1", Chunks: []string{"a"}},
	}
	p2 := &callTrackProvider{
		Provider: &testutil.OkProvider{ProviderKey: "p2", ProviderModel: "m2", Chunks: []string{"b"}},
	}

	aps := []chat.ActiveProvider{sampleAP("p1", "m1"), sampleAP("p2", "m2")}
	registry := map[string]chat.Provider{"p1/m1": p1, "p2/m2": p2}
	r := newRouter(100*time.Millisecond, aps, registry)

	// Request 1 → should hit p1 (index 0).
	ch1, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	drain(ch1) //nolint:errcheck

	// Request 2 → should hit p2 (index 1).
	ch2, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	drain(ch2) //nolint:errcheck

	// Request 3 → wraps around to p1 again.
	ch3, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	drain(ch3) //nolint:errcheck

	if p1.callCount != 2 {
		t.Errorf("p1 call count: got %d, want 2", p1.callCount)
	}
	if p2.callCount != 1 {
		t.Errorf("p2 call count: got %d, want 1", p2.callCount)
	}
}

func TestRouter_RoundRobin_DoesNotAdvanceOnFailure(t *testing.T) {
	// p1 always fails. Round-robin must not advance so the next request also starts at p1.
	p1 := &callTrackProvider{
		Provider: &testutil.EmptyProvider{ProviderKey: "p1", ProviderModel: "m1"},
	}
	p2 := &callTrackProvider{
		Provider: &testutil.OkProvider{ProviderKey: "p2", ProviderModel: "m2", Chunks: []string{"ok"}},
	}

	aps := []chat.ActiveProvider{sampleAP("p1", "m1"), sampleAP("p2", "m2")}
	registry := map[string]chat.Provider{"p1/m1": p1, "p2/m2": p2}
	r := newRouter(100*time.Millisecond, aps, registry)

	// Both requests should exhaust p1, fall back to p2, and p2 advances the index.
	for i := 0; i < 2; i++ {
		ch, err := r.Handle(context.Background(), nil)
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		drain(ch) //nolint:errcheck
	}
	// p1 should have been called twice (both requests start there, both fail).
	if p1.callCount != 2 {
		t.Errorf("p1 call count: got %d, want 2", p1.callCount)
	}
}

// ─── probe timeout ────────────────────────────────────────────────────────────

func TestRouter_ProbeTimeout_FallsBackToNextProvider(t *testing.T) {
	slow := &testutil.SlowProvider{
		ProviderKey:   "p1",
		ProviderModel: "m1",
		Delay:         500 * time.Millisecond, // longer than probe timeout
		Chunks:        []string{"never"},
	}
	fast := &testutil.OkProvider{
		ProviderKey:   "p2",
		ProviderModel: "m2",
		Chunks:        []string{"fast ok"},
	}

	aps := []chat.ActiveProvider{sampleAP("p1", "m1"), sampleAP("p2", "m2")}
	registry := map[string]chat.Provider{"p1/m1": slow, "p2/m2": fast}
	r := newRouter(50*time.Millisecond, aps, registry) // 50ms probe timeout

	ch, err := r.Handle(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := drain(ch)
	if got != "fast ok" {
		t.Errorf("got %q, want %q", got, "fast ok")
	}
}

// ─── goroutine cleanup (FD-leak equivalent) ───────────────────────────────────

func TestRouter_CancelledProbes_CleanUpGoroutines(t *testing.T) {
	const N = 20

	// Build a single-provider router where the provider is always slow (times out).
	// Use SlowProvider.Done to detect when each goroutine exits.
	doneChans := make([]chan struct{}, N)
	registry := make(map[string]chat.Provider)

	for i := 0; i < N; i++ {
		key := fmt.Sprintf("p%d", i)
		model := "m"
		done := make(chan struct{})
		doneChans[i] = done
		sp := &testutil.SlowProvider{
			ProviderKey:   key,
			ProviderModel: model,
			Delay:         500 * time.Millisecond,
			Done:          done,
		}
		registry[key+"/"+model] = sp
	}

	// Only the first provider is used for this test; use a single-AP router.
	// We call Handle N times sequentially using distinct routers (each with one slow provider).
	for i := 0; i < N; i++ {
		key := fmt.Sprintf("p%d", i)
		model := "m"
		ap := sampleAP(key, model)
		r := newRouter(
			10*time.Millisecond, // very short probe timeout
			[]chat.ActiveProvider{ap},
			map[string]chat.Provider{key + "/" + model: registry[key+"/"+model]},
		)
		_, err := r.Handle(context.Background(), nil)
		if err == nil {
			t.Errorf("router %d: expected error from slow provider", i)
		}
	}

	// All N slow goroutines should exit within a generous deadline.
	deadline := time.After(2 * time.Second)
	for i, done := range doneChans {
		select {
		case <-done:
		case <-deadline:
			t.Errorf("goroutine %d did not exit after probe cancel", i)
		}
	}
}

// ─── context cancellation mid-stream ─────────────────────────────────────────

func TestRouter_ContextCancelled_MidStream(t *testing.T) {
	// A provider that produces chunks slowly so we can cancel mid-way.
	p := &testutil.SlowProvider{
		ProviderKey:   "p1",
		ProviderModel: "m1",
		Delay:         0, // first chunk immediate
		Chunks:        []string{"chunk1"},
	}
	ap := sampleAP("p1", "m1")
	r := newRouter(100*time.Millisecond, []chat.ActiveProvider{ap}, map[string]chat.Provider{"p1/m1": p})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := r.Handle(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read first chunk then cancel.
	<-ch
	cancel()

	// Drain remaining; channel must close (not block forever).
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel closed cleanly
			}
		case <-timeout:
			t.Fatal("channel did not close after context cancellation")
		}
	}
}

// ─── callTrackProvider ────────────────────────────────────────────────────────
// A wrapper that counts how many times Chat() is called and optionally records
// whether the provider was ever invoked at all.

type callTrackProvider struct {
	chat.Provider
	callCount int
	called    *bool
}

func (c *callTrackProvider) Chat(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	c.callCount++
	if c.called != nil {
		*c.called = true
	}
	return c.Provider.Chat(ctx, msgs)
}

// ─── router with zero active providers ─────────────────────────────���─────────

func TestRouter_NoProviders_ReturnsError(t *testing.T) {
	// listActive returns empty slice → Snapshot(0) and Advance(0,0) branches hit.
	r := chat.New(0,
		func(_ context.Context) ([]chat.ActiveProvider, error) { return nil, nil },
		func(_, _ string) (chat.Provider, error) {
			return &testutil.OkProvider{ProviderKey: "p", ProviderModel: "m", Chunks: []string{"x"}}, nil
		},
	)
	_, err := r.Handle(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error with no providers, got nil")
	}
}
