package chat_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/testutil"
)

// TestRouter_RoundRobin_ConcurrentHandle launches N goroutines all calling
// Handle() simultaneously. The -race detector validates the round-robin
// counter is always accessed under the mutex.
func TestRouter_RoundRobin_ConcurrentHandle(t *testing.T) {
	const goroutines = 50

	aps := []chat.ActiveProvider{
		{Key: "p1", Name: "P1", DefaultModel: "m1", Models: []string{"m1"}},
		{Key: "p2", Name: "P2", DefaultModel: "m2", Models: []string{"m2"}},
		{Key: "p3", Name: "P3", DefaultModel: "m3", Models: []string{"m3"}},
	}
	registry := map[string]chat.Provider{
		"p1/m1": &testutil.OkProvider{ProviderKey: "p1", ProviderModel: "m1", Chunks: []string{"ok"}},
		"p2/m2": &testutil.OkProvider{ProviderKey: "p2", ProviderModel: "m2", Chunks: []string{"ok"}},
		"p3/m3": &testutil.OkProvider{ProviderKey: "p3", ProviderModel: "m3", Chunks: []string{"ok"}},
	}

	r := newRouter(200*time.Millisecond, aps, registry)

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, err := r.Handle(context.Background(), nil)
			if err != nil {
				errs <- fmt.Errorf("Handle: %w", err)
				return
			}
			for range ch { /* drain */ }
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestRouter_RoundRobin_IndexStaysInRange verifies the round-robin index never
// produces an out-of-range offset when many goroutines succeed concurrently.
func TestRouter_RoundRobin_IndexStaysInRange(t *testing.T) {
	const goroutines = 100
	const numProviders = 5

	aps := make([]chat.ActiveProvider, numProviders)
	registry := make(map[string]chat.Provider)
	for i := 0; i < numProviders; i++ {
		key := fmt.Sprintf("p%d", i)
		model := fmt.Sprintf("m%d", i)
		aps[i] = chat.ActiveProvider{Key: key, Name: key, DefaultModel: model, Models: []string{model}}
		registry[key+"/"+model] = &testutil.OkProvider{
			ProviderKey:   key,
			ProviderModel: model,
			Chunks:        []string{"chunk"},
		}
	}

	r := newRouter(200*time.Millisecond, aps, registry)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, err := r.Handle(context.Background(), nil)
			if err != nil {
				return
			}
			for range ch { /* drain */ }
		}()
	}
	wg.Wait()
	// If we reach here without the race detector firing, the test passes.
}

// TestRouter_ConcurrentHandleWithFallback tests concurrent requests where some
// providers fail, ensuring the router is race-free under fallback conditions.
func TestRouter_ConcurrentHandleWithFallback(t *testing.T) {
	const goroutines = 30

	aps := []chat.ActiveProvider{
		{Key: "unstable", Name: "Unstable", DefaultModel: "um", Models: []string{"um"}},
		{Key: "stable", Name: "Stable", DefaultModel: "sm", Models: []string{"sm"}},
	}

	registry := map[string]chat.Provider{
		// unstable always fails (empty stream).
		"unstable/um": &testutil.EmptyProvider{ProviderKey: "unstable", ProviderModel: "um"},
		// stable always succeeds.
		"stable/sm": &testutil.OkProvider{ProviderKey: "stable", ProviderModel: "sm", Chunks: []string{"stable"}},
	}

	r := newRouter(200*time.Millisecond, aps, registry)

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ch, err := r.Handle(context.Background(), nil)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d Handle: %w", n, err)
				return
			}
			for range ch { /* drain */ }
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
