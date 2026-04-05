package gemini

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

func sampleParams() provparams.Params { return provparams.DefaultParams("gemini-1.5-flash") }

// geminiSSE emits one Gemini streaming candidate chunk.
func geminiSSE(text string) string {
	data := fmt.Sprintf(
		`{"candidates":[{"content":{"parts":[{"text":%q}],"role":"model"}}]}`,
		text,
	)
	return "data: " + data + "\n\n"
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
		fmt.Fprint(w, geminiSSE("gemini "))  //nolint:errcheck
		fmt.Fprint(w, geminiSSE("reply"))    //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "gemini reply" {
		t.Errorf("got %q, want %q", got, "gemini reply")
	}
}

// TestChat_RoleMapping verifies "assistant"→"model" mapping.
func TestChat_RoleMapping(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody) //nolint:errcheck
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, geminiSSE("ok")) //nolint:errcheck
	}))
	defer srv.Close()

	msgs := []chat.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {}

	contents, ok := gotBody["contents"].([]any)
	if !ok || len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %v", gotBody["contents"])
	}
	second := contents[1].(map[string]any)
	if second["role"] != "model" {
		t.Errorf("assistant role must map to 'model', got %q", second["role"])
	}
}

// TestChat_SystemInstruction verifies system messages are extracted to systemInstruction.
func TestChat_SystemInstruction(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody) //nolint:errcheck
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, geminiSSE("ok")) //nolint:errcheck
	}))
	defer srv.Close()

	msgs := []chat.ChatMessage{
		{Role: "system", Content: "Be concise."},
		{Role: "user", Content: "Hello"},
	}
	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {}

	si, ok := gotBody["systemInstruction"]
	if !ok {
		t.Fatal("systemInstruction missing from request body")
	}
	siMap := si.(map[string]any)
	parts := siMap["parts"].([]any)
	if len(parts) == 0 {
		t.Fatal("systemInstruction.parts is empty")
	}
	text := parts[0].(map[string]any)["text"]
	if text != "Be concise." {
		t.Errorf("systemInstruction text: got %v", text)
	}

	// System message must NOT appear in contents.
	contents := gotBody["contents"].([]any)
	if len(contents) != 1 {
		t.Errorf("expected 1 content (user only), got %d", len(contents))
	}
}

// TestChat_URLContainsModelAndKey verifies the URL format.
func TestChat_URLContainsModelAndKey(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, geminiSSE("ok")) //nolint:errcheck
	}))
	defer srv.Close()

	params := provparams.DefaultParams("gemini-1.5-flash")
	c := newClient(srv.URL, "my-api-key", params, &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range ch {}

	if !strings.Contains(gotPath, "gemini-1.5-flash") {
		t.Errorf("URL should contain model name: %s", gotPath)
	}
	if !strings.Contains(gotPath, "my-api-key") {
		t.Errorf("URL should contain API key: %s", gotPath)
	}
	if !strings.Contains(gotPath, "alt=sse") {
		t.Errorf("URL should contain alt=sse: %s", gotPath)
	}
}

func TestChat_RequestHeaders(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, geminiSSE("ok")) //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q", gotUA)
	}
}

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "quota exceeded", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Errorf("expected 429 error, got %v", err)
	}
}

func TestChat_FDLeak_CancelledProbes(t *testing.T) {
	const N = 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, geminiSSE("hi")) //nolint:errcheck
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

// TestChat_MultiPartResponse verifies parts are joined correctly.
func TestChat_MultiPartResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Single candidate with multiple parts
		data := `{"candidates":[{"content":{"parts":[{"text":"foo"},{"text":"bar"}],"role":"model"}}]}`
		fmt.Fprintf(w, "data: %s\n\n", data) //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "foobar" {
		t.Errorf("got %q, want %q", got, "foobar")
	}
}

func TestNew_Accessors(t *testing.T) {
	params := provparams.DefaultParams("gemini-pro")
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
	if a.Model() != "gemini-pro" {
		t.Errorf("Model() = %q, want %q", a.Model(), "gemini-pro")
	}
	if a.Name() == "" {
		t.Error("Name() is empty")
	}
	if a.Key() == "" {
		t.Error("Key() is empty")
	}
}
