package groq

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

func sampleParams() provparams.Params { return provparams.DefaultParams("llama-3.3-70b") }

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

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk("groq "))    //nolint:errcheck
		fmt.Fprint(w, sseChunk("response")) //nolint:errcheck
		fmt.Fprint(w, "data: [DONE]\n\n")  //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := collect(t, ch)
	if got != "groq response" {
		t.Errorf("got %q, want %q", got, "groq response")
	}
}

func TestChat_RequestHeaders(t *testing.T) {
	var gotUA, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "groq-key", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q", gotUA)
	}
	if gotAuth != "Bearer groq-key" {
		t.Errorf("Authorization: got %q", gotAuth)
	}
}

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for 401")
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
