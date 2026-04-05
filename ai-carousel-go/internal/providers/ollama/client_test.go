package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/chat"
	"github.com/thotenn/aicarousel-go/internal/providers/provparams"
	"github.com/thotenn/aicarousel-go/testutil"
)

func sampleParams() provparams.Params { return provparams.DefaultParams("llama3") }

func sseChunk(content string) string {
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", content)
}

func collect(t *testing.T, ch <-chan chat.StreamChunk) string {
	t.Helper()
	var b strings.Builder
	for c := range ch {
		b.WriteString(c.Text)
	}
	return b.String()
}

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk("local"))   //nolint:errcheck
		fmt.Fprint(w, sseChunk(" llm"))    //nolint:errcheck
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/chat/completions", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "local llm" {
		t.Errorf("got %q, want %q", got, "local llm")
	}
}

// TestChat_UsesMaxTokens verifies that Ollama uses "max_tokens" (not "max_completion_tokens").
func TestChat_UsesMaxTokens(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/chat/completions", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {}

	if _, ok := body["max_tokens"]; !ok {
		t.Error("request body must contain 'max_tokens'")
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Error("request body must NOT contain 'max_completion_tokens' for Ollama")
	}
}

// TestChat_MalformedJSONLine_Skipped verifies that malformed SSE JSON is silently skipped.
func TestChat_MalformedJSONLine_Skipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {not valid json}\n\n") //nolint:errcheck // malformed //nolint:errcheck
		fmt.Fprint(w, sseChunk("good"))             //nolint:errcheck
		fmt.Fprint(w, "data: [DONE]\n\n")           //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/chat/completions", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "good" {
		t.Errorf("got %q, want %q", got, "good")
	}
}

func TestChat_RequestHeaders(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/chat/completions", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q", gotUA)
	}
}

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/chat/completions", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got %v", err)
	}
}

func TestChat_FDLeak_CancelledProbes(t *testing.T) {
	const N = 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk("ok")) //nolint:errcheck
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	tt := &testutil.TrackTransport{}
	httpClient := &http.Client{Transport: tt}

	for i := 0; i < N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		c := newClient(srv.URL+"/v1/chat/completions", sampleParams(), httpClient)
		ch, err := c.Chat(ctx, nil)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		<-ch
		cancel()
		for range ch {}
	}

	if got := tt.Closes(); got != N {
		t.Errorf("FD-leak: Body.Close() called %d times, want %d", got, N)
	}
}

func TestNew_Accessors(t *testing.T) {
	params := provparams.DefaultParams("llama3")
	c := New("", params) // ollama ignores apiKey
	if c == nil {
		t.Fatal("New returned nil")
	}
	type accessor interface {
		Name() string
		Key() string
		Model() string
	}
	a, ok := c.(accessor)
	if !ok {
		t.Fatal("provider does not implement accessor interface")
	}
	if a.Model() != "llama3" {
		t.Errorf("Model() = %q, want %q", a.Model(), "llama3")
	}
	if a.Name() == "" {
		t.Error("Name() is empty")
	}
	if a.Key() == "" {
		t.Error("Key() is empty")
	}
}
