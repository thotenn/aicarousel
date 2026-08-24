package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel/internal/chat"
)

// ── stub router ───────────────────────────────────────────────────────────────

type stubRouter struct {
	chunks  []string
	err     error
	gotOpts *chat.Options
}

func (s *stubRouter) Handle(_ context.Context, _ []chat.ChatMessage, opts chat.Options) (<-chan chat.StreamChunk, error) {
	if s.gotOpts != nil {
		*s.gotOpts = opts
	}
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan chat.StreamChunk, len(s.chunks))
	for _, t := range s.chunks {
		ch <- chat.StreamChunk{Text: t}
	}
	close(ch)
	return ch, nil
}

func okRouter(chunks ...string) chatRouter { return &stubRouter{chunks: chunks} }
func errRouter(msg string) chatRouter      { return &stubRouter{err: errTest(msg)} }

type errTest string

func (e errTest) Error() string { return string(e) }

// ── helpers ───────────────────────────────────────────────────────────────────

func post(t *testing.T, h *Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, r)
	return w
}

func get(t *testing.T, h *Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, r)
	return w
}

// ── POST /v1/chat/completions ─────────────────────────────────────────────────

func TestChatCompletions_MissingMessages_400(t *testing.T) {
	h := New(okRouter())
	w := post(t, h, "/v1/chat/completions", map[string]any{"model": "aicarousel"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Error.Type != "invalid_request_error" {
		t.Errorf("error.type: %q", resp.Error.Type)
	}
}

func TestChatCompletions_Streaming_Default(t *testing.T) {
	h := New(okRouter("Hello", " world"))
	w := post(t, h, "/v1/chat/completions", map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type: got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Hello") {
		t.Errorf("missing Hello in body: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("missing [DONE] in body: %s", body)
	}
}

func TestChatCompletions_NonStreaming(t *testing.T) {
	h := New(okRouter("Hello", " world"))
	w := post(t, h, "/v1/chat/completions", map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp struct {
		Object  string `json:"object"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object: %q", resp.Object)
	}
	if resp.Choices[0].Message.Content != "Hello world" {
		t.Errorf("content: %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason: %q", resp.Choices[0].FinishReason)
	}
	if resp.Usage.CompletionTokens == 0 {
		t.Error("completion_tokens should be > 0")
	}
}

func TestChatCompletions_RouterError_503(t *testing.T) {
	h := New(errRouter("all down"))
	w := post(t, h, "/v1/chat/completions", map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", w.Code)
	}
}

func TestChatCompletions_StreamSSEHeaders(t *testing.T) {
	h := New(okRouter("hi"))
	w := post(t, h, "/v1/chat/completions", map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	headers := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	}
	for k, want := range headers {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s: got %q, want %q", k, got, want)
		}
	}
}

func TestChatCompletions_ModelDefault(t *testing.T) {
	h := New(okRouter("hi"))
	w := post(t, h, "/v1/chat/completions", map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   false,
	})
	var resp struct {
		Model string `json:"model"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Model != "aicarousel" {
		t.Errorf("model default: got %q", resp.Model)
	}
}

// ── GET /v1/models ────────────────────────────────────────────────────────────

func TestModels_200(t *testing.T) {
	h := New(okRouter())
	w := get(t, h, "/v1/models")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Object != "list" {
		t.Errorf("object: %q", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Error("models list is empty")
	}
	// aicarousel must be in list
	found := false
	for _, m := range resp.Data {
		if m.ID == "aicarousel" {
			found = true
		}
	}
	if !found {
		t.Error("aicarousel not in models list")
	}
}

// ── GET /v1/models/{id} ───────────────────────────────────────────────────────

func TestModelInfo_ReturnsID(t *testing.T) {
	h := New(okRouter())
	w := get(t, h, "/v1/models/gpt-4")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.ID != "gpt-4" {
		t.Errorf("id: got %q, want gpt-4", resp.ID)
	}
	if resp.Object != "model" {
		t.Errorf("object: %q", resp.Object)
	}
}

func TestModelInfo_EmptyID_NotFound(t *testing.T) {
	// PathValue("id") returns "" when the segment is stripped — handler should 404.
	h := New(okRouter())
	// Directly invoke with an empty path value via the full path (Go mux won't
	// match /v1/models/ with a trailing slash to {id}, so we use a unit-level
	// call via the mux with a path that yields an empty value after TrimPrefix).
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/models/", nil)
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, r)
	// /v1/models/ falls through to the /v1/models handler (no trailing slash
	// match for {id}), so we test the TrimPrefix branch by calling modelInfo directly.
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/v1/models/", nil)
	// Override PathValue by using a custom request context isn't straightforward;
	// instead verify the handler returns 404 for an empty segment.
	h.modelInfo(w2, r2) // r2.PathValue("id") == "" → 404
	if w2.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w2.Code)
	}
}

// ── request sampling params ──────────────────────────────────────────────────

// TestChatCompletions_ForwardsSamplingParams verifies the caller's params reach
// the router instead of being silently dropped.
func TestChatCompletions_ForwardsSamplingParams(t *testing.T) {
	var got chat.Options
	h := New(&stubRouter{chunks: []string{"hi"}, gotOpts: &got})

	post(t, h, "/v1/chat/completions", map[string]any{
		"model":       "aicarousel",
		"messages":    []map[string]string{{"role": "user", "content": "hi"}},
		"stream":      false,
		"temperature": 0.2,
		"top_p":       0.5,
		"max_tokens":  500,
		"stop":        "<end>",
	})

	if got.Temperature == nil || *got.Temperature != 0.2 {
		t.Errorf("temperature = %v, want 0.2", got.Temperature)
	}
	if got.TopP == nil || *got.TopP != 0.5 {
		t.Errorf("top_p = %v, want 0.5", got.TopP)
	}
	if got.MaxTokens == nil || *got.MaxTokens != 500 {
		t.Errorf("max_tokens = %v, want 500", got.MaxTokens)
	}
	if got.Stop == nil || *got.Stop != "<end>" {
		t.Errorf("stop = %v, want \"<end>\"", got.Stop)
	}
}

// TestChatCompletions_OmittedParamsStayNil verifies unset params leave the
// provider defaults untouched.
func TestChatCompletions_OmittedParamsStayNil(t *testing.T) {
	var got chat.Options
	h := New(&stubRouter{chunks: []string{"hi"}, gotOpts: &got})

	post(t, h, "/v1/chat/completions", map[string]any{
		"model":    "aicarousel",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   false,
	})

	if got.Temperature != nil || got.TopP != nil || got.MaxTokens != nil || got.Stop != nil {
		t.Errorf("omitted params must stay nil, got %+v", got)
	}
}
