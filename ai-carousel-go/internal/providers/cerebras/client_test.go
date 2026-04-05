package cerebras

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/chat"
	"github.com/thotenn/aicarousel-go/internal/providers/provparams"
	"github.com/thotenn/aicarousel-go/testutil"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func sampleParams() provparams.Params { return provparams.DefaultParams("test-model") }

func sseChunk(content string) string {
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", content)
}

func collect(t *testing.T, ch <-chan chat.StreamChunk) (string, error) {
	t.Helper()
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

// ─── success ─────────────────────────────────────────────────────────────────

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk("hello"))    //nolint:errcheck
		fmt.Fprint(w, sseChunk(" world"))   //nolint:errcheck
		fmt.Fprint(w, "data: [DONE]\n\n")  //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := collect(t, ch)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

// ─── request headers ─────────────────────────────────────────────────────────

func TestChat_RequestHeaders(t *testing.T) {
	var gotUA, gotAuth, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "test-api-key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q, want AICarousel-Go/...", gotUA)
	}
	if gotAuth != "Bearer test-api-key" {
		t.Errorf("Authorization: got %q, want Bearer test-api-key", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", gotCT)
	}
}

// ─── non-200 response ─────────────────────────────────────────────────────────

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention status code: %v", err)
	}
}

// ─── empty delta (skipped) ────────────────────────────────────────────────────

func TestChat_EmptyDelta_Skipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// First chunk: empty content (role line at stream start)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n") //nolint:errcheck
		fmt.Fprint(w, sseChunk("actual")) //nolint:errcheck
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := collect(t, ch)
	if got != "actual" {
		t.Errorf("got %q, want %q", got, "actual")
	}
}

// ─── context cancellation ─────────────────────────────────────────────────────

func TestChat_ContextCancelled_ChannelCloses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprint(w, sseChunk("first")) //nolint:errcheck
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-ch     // read first chunk
	cancel() // cancel context
	for range ch { /* drain */ }
}

// ─── FD-leak: N=100 cancelled probes → N Body.Close() calls ──────────────────

func TestChat_FDLeak_CancelledProbes(t *testing.T) {
	const N = 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk("hi")) //nolint:errcheck
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	tt := &testutil.TrackTransport{}
	httpClient := &http.Client{Transport: tt}

	for i := 0; i < N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		c := newClient(srv.URL, "key", sampleParams(), httpClient)
		ch, err := c.Chat(ctx, nil)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		<-ch     // wait for first chunk
		cancel() // trigger cancellation
		for range ch { /* drain so goroutine exits */ }
	}

	if got := tt.Closes(); got != N {
		t.Errorf("FD-leak: Body.Close() called %d times, want %d", got, N)
	}
}

// ─── channel buffer ───────────────────────────────────────────────────────────

func TestChat_ChannelBufferIs10(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 10; i++ {
			fmt.Fprint(w, sseChunk(fmt.Sprintf("chunk%d", i))) //nolint:errcheck
		}
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap(ch) != 10 {
		t.Errorf("channel buffer: got %d, want 10", cap(ch))
	}
	for range ch {}
}

func TestNew_Accessors(t *testing.T) {
	params := provparams.DefaultParams("my-model")
	c := New("sk-test", params)
	if c == nil {
		t.Fatal("New returned nil")
	}
	// Use interface methods via a type assertion to the concrete *client.
	type accessor interface {
		Name() string
		Key() string
		Model() string
	}
	a, ok := c.(accessor)
	if !ok {
		t.Fatal("provider does not implement accessor interface")
	}
	if a.Model() != "my-model" {
		t.Errorf("Model() = %q, want %q", a.Model(), "my-model")
	}
	if a.Name() == "" {
		t.Error("Name() is empty")
	}
	if a.Key() == "" {
		t.Error("Key() is empty")
	}
}
