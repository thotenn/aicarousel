package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/internal/providers/provparams"
	"github.com/thotenn/aicarousel/testutil"
)

func sampleParams() provparams.Params { return provparams.DefaultParams("openai/gpt-4o") }

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
		fmt.Fprint(w, sseChunk("or"))  //nolint:errcheck
		fmt.Fprint(w, sseChunk(" ok")) //nolint:errcheck
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "or ok" {
		t.Errorf("got %q, want %q", got, "or ok")
	}
}

func TestChat_RequestHeaders(t *testing.T) {
	var gotUA, gotReferer, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q", gotUA)
	}
	if gotReferer != "https://aicarousel.local" {
		t.Errorf("HTTP-Referer: got %q", gotReferer)
	}
	if gotTitle != "AICarousel" {
		t.Errorf("X-Title: got %q", gotTitle)
	}
}

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 error, got %v", err)
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
		c := newClient(srv.URL, "key", sampleParams(), httpClient)
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
