package zai

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

func sampleParams() provparams.Params { return provparams.DefaultParams("claude-3-5-sonnet") }

// anthropicSSE emits a content_block_delta event with the given text.
func anthropicSSE(text string) string {
	data := fmt.Sprintf(
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":%q}}`,
		text,
	)
	return "event: content_block_delta\ndata: " + data + "\n\n"
}

func collect(t *testing.T, ch <-chan chat.StreamChunk) string {
	t.Helper()
	var b strings.Builder
	for c := range ch {
		b.WriteString(c.Text)
	}
	return b.String()
}

// ─── success ─────────────────────────────────────────────────────────────────

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSE("hello"))                                              //nolint:errcheck
		fmt.Fprint(w, anthropicSSE(" zai"))                                               //nolint:errcheck
		// message_stop — should be ignored (not text_delta)
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/messages", "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "hello zai" {
		t.Errorf("got %q, want %q", got, "hello zai")
	}
}

// TestChat_OnlyTextDelta verifies that non-text events are filtered out.
func TestChat_OnlyTextDelta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// message_start — must be ignored
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n") //nolint:errcheck
		// content_block_start — must be ignored
		fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\"}\n\n") //nolint:errcheck
		// actual text
		fmt.Fprint(w, anthropicSSE("only this")) //nolint:errcheck
		// ping — must be ignored
		fmt.Fprint(w, "event: ping\ndata: {\"type\":\"ping\"}\n\n") //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/messages", "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "only this" {
		t.Errorf("got %q, want %q", got, "only this")
	}
}

// TestChat_SystemExtraction verifies system messages are separated from the conversation.
func TestChat_SystemExtraction(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody) //nolint:errcheck
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSE("ok")) //nolint:errcheck
	}))
	defer srv.Close()

	msgs := []chat.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
	}
	c := newClient(srv.URL+"/v1/messages", "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {}

	if gotBody["system"] != "You are helpful." {
		t.Errorf("system field: got %v", gotBody["system"])
	}
	msgList, ok := gotBody["messages"].([]any)
	if !ok || len(msgList) != 1 {
		t.Errorf("messages: expected 1 non-system message, got %v", gotBody["messages"])
	}
}

// TestChat_RequestHeaders verifies Z.ai-specific auth headers.
func TestChat_RequestHeaders(t *testing.T) {
	var gotUA, gotKey, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSE("ok")) //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/messages", "zai-secret", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q", gotUA)
	}
	if gotKey != "zai-secret" {
		t.Errorf("x-api-key: got %q", gotKey)
	}
	if gotVersion != "2023-06-01" {
		t.Errorf("anthropic-version: got %q", gotVersion)
	}
}

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/v1/messages", "key", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 error, got %v", err)
	}
}

func TestChat_FDLeak_CancelledProbes(t *testing.T) {
	const N = 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSE("hi")) //nolint:errcheck
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	tt := &testutil.TrackTransport{}
	httpClient := &http.Client{Transport: tt}

	for i := 0; i < N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		c := newClient(srv.URL+"/v1/messages", "key", sampleParams(), httpClient)
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
	params := provparams.DefaultParams("glm-4")
	c := New("sk-test", params)
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
	if a.Model() != "glm-4" {
		t.Errorf("Model() = %q, want %q", a.Model(), "glm-4")
	}
	if a.Name() == "" {
		t.Error("Name() is empty")
	}
	if a.Key() == "" {
		t.Error("Key() is empty")
	}
}
