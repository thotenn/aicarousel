package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/internal/config"
	"github.com/thotenn/aicarousel/internal/providers/provparams"
	"github.com/thotenn/aicarousel/testutil"
)

func sampleParams() provparams.Params { return provparams.DefaultParams("llama3") }

// ndjsonChunk renders one line of Ollama's native stream.
func ndjsonChunk(content string) string {
	return fmt.Sprintf("{\"message\":{\"role\":\"assistant\",\"content\":%q},\"done\":false}\n", content)
}

const ndjsonDone = "{\"message\":{\"role\":\"assistant\",\"content\":\"\"},\"done\":true}\n"

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
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, ndjsonChunk("local")) //nolint:errcheck
		fmt.Fprint(w, ndjsonChunk(" llm"))  //nolint:errcheck
		fmt.Fprint(w, ndjsonDone)           //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), []chat.ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "local llm" {
		t.Errorf("got %q, want %q", got, "local llm")
	}
}

// TestChat_UsesNativeOptions verifies generation settings go in the native
// "options" object (the only place num_ctx can be set), not at the top level.
func TestChat_UsesNativeOptions(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, ndjsonDone) //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {
	}

	opts, ok := body["options"].(map[string]any)
	if !ok {
		t.Fatal("request body must contain an 'options' object")
	}
	if _, ok := opts["num_ctx"]; !ok {
		t.Error("options must contain 'num_ctx'")
	}
	if got := opts["num_predict"]; got != float64(provparams.DefaultMaxTokens) {
		t.Errorf("num_predict = %v, want %v", got, provparams.DefaultMaxTokens)
	}
	if _, ok := body["max_tokens"]; ok {
		t.Error("request body must NOT contain a top-level 'max_tokens'")
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Error("request body must NOT contain 'max_completion_tokens'")
	}
}

// TestChat_NumCtxFromConfig verifies OLLAMA_NUM_CTX reaches the request, and
// that an unset/invalid value falls back to the default.
func TestChat_NumCtxFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		configVal int
		want      float64
	}{
		{"from config", 16384, 16384},
		{"unset falls back", 0, defaultNumCtx},
		{"negative falls back", -1, defaultNumCtx},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := config.Cfg.OllamaNumCtx
			config.Cfg.OllamaNumCtx = tt.configVal
			defer func() { config.Cfg.OllamaNumCtx = original }()

			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
				fmt.Fprint(w, ndjsonDone)             //nolint:errcheck
			}))
			defer srv.Close()

			c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
			ch, _ := c.Chat(context.Background(), nil)
			for range ch {
			}

			opts := body["options"].(map[string]any)
			if got := opts["num_ctx"]; got != tt.want {
				t.Errorf("num_ctx = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestChat_StopSequence verifies a stop value is passed as an array.
func TestChat_StopSequence(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		fmt.Fprint(w, ndjsonDone)             //nolint:errcheck
	}))
	defer srv.Close()

	params := sampleParams()
	stop := "<end>"
	params.Stop = &stop

	c := newClient(srv.URL+"/api/chat", params, &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {
	}

	opts := body["options"].(map[string]any)
	got, ok := opts["stop"].([]any)
	if !ok || len(got) != 1 || got[0] != "<end>" {
		t.Errorf("stop = %v, want [\"<end>\"]", opts["stop"])
	}
}

// TestChat_ThinkingIgnored verifies reasoning output never reaches the caller.
func TestChat_ThinkingIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{\"message\":{\"content\":\"\",\"thinking\":\"secret reasoning\"},\"done\":false}\n") //nolint:errcheck
		fmt.Fprint(w, ndjsonChunk("answer"))                                                                 //nolint:errcheck
		fmt.Fprint(w, ndjsonDone)                                                                            //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
	ch, err := c.Chat(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(t, ch); got != "answer" {
		t.Errorf("got %q, want %q", got, "answer")
	}
}

// TestChat_MalformedJSONLine_Skipped verifies that malformed NDJSON is silently skipped.
func TestChat_MalformedJSONLine_Skipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{not valid json}\n") //nolint:errcheck
		fmt.Fprint(w, ndjsonChunk("good"))  //nolint:errcheck
		fmt.Fprint(w, ndjsonDone)           //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
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
		fmt.Fprint(w, ndjsonDone) //nolint:errcheck
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
	ch, _ := c.Chat(context.Background(), nil)
	for range ch {
	}

	if !strings.HasPrefix(gotUA, "AICarousel-Go/") {
		t.Errorf("User-Agent: got %q", gotUA)
	}
}

func TestChat_Non200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
	_, err := c.Chat(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got %v", err)
	}
}

func TestChat_FDLeak_CancelledProbes(t *testing.T) {
	const N = 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		fmt.Fprint(w, ndjsonChunk("ok")) //nolint:errcheck
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	tt := &testutil.TrackTransport{}
	httpClient := &http.Client{Transport: tt}

	for i := 0; i < N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		c := newClient(srv.URL+"/api/chat", sampleParams(), httpClient)
		ch, err := c.Chat(ctx, nil)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		<-ch
		cancel()
		for range ch {
		}
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

// TestChat_KeepAlive verifies OLLAMA_KEEP_ALIVE reaches the native payload in
// the type Ollama actually parses: a number for seconds (negative = forever), a
// string for a Go duration. Sending "-1" as a string would fail ParseDuration on
// Ollama's side and take the whole provider down.
func TestChat_KeepAlive(t *testing.T) {
	tests := []struct {
		name      string
		configVal string
		wantSet   bool
		want      any
	}{
		{"unset is omitted", "", false, nil},
		{"forever goes as a number", "-1", true, float64(-1)},
		{"seconds go as a number", "3600", true, float64(3600)},
		{"zero unloads immediately", "0", true, float64(0)},
		{"duration goes as a string", "30m", true, "30m"},
		{"whitespace is trimmed", "  30m  ", true, "30m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := config.Cfg.OllamaKeepAlive
			config.Cfg.OllamaKeepAlive = tt.configVal
			defer func() { config.Cfg.OllamaKeepAlive = original }()

			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
				fmt.Fprint(w, ndjsonDone)             //nolint:errcheck
			}))
			defer srv.Close()

			c := newClient(srv.URL+"/api/chat", sampleParams(), &http.Client{})
			ch, _ := c.Chat(context.Background(), nil)
			for range ch {
			}

			got, ok := body["keep_alive"]
			if ok != tt.wantSet {
				t.Fatalf("keep_alive present = %v, want %v", ok, tt.wantSet)
			}
			if tt.wantSet && got != tt.want {
				t.Errorf("keep_alive = %#v, want %#v", got, tt.want)
			}
		})
	}
}
