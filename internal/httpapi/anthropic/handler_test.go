package anthropic

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
	chunks []string
	err    error
}

func (s *stubRouter) Handle(_ context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
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

type errTest string

func (e errTest) Error() string { return string(e) }

func okRouter(chunks ...string) chatRouter { return &stubRouter{chunks: chunks} }
func errRouter(msg string) chatRouter      { return &stubRouter{err: errTest(msg)} }

// ── helpers ───────────────────────────────────────────────────────────────────

func doPost(t *testing.T, h *Handler, path string, body any) *httptest.ResponseRecorder {
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

// ── POST /v1/messages ─────────────────────────────────────────────────────────

func TestMessages_MissingMessages_400(t *testing.T) {
	h := New(okRouter())
	w := doPost(t, h, "/v1/messages", map[string]any{"model": "claude", "max_tokens": 100})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
	assertAnthropicError(t, w.Body.Bytes(), "invalid_request_error")
}

func TestMessages_MissingMaxTokens_400(t *testing.T) {
	h := New(okRouter())
	w := doPost(t, h, "/v1/messages", map[string]any{
		"model":    "claude",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
	assertAnthropicError(t, w.Body.Bytes(), "invalid_request_error")
}

func TestMessages_NonStreaming_Default(t *testing.T) {
	h := New(okRouter("Hello", " world"))
	w := doPost(t, h, "/v1/messages", map[string]any{
		"model":      "claude-3-5-sonnet",
		"max_tokens": 100,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: %q", ct)
	}
	var resp struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Role        string `json:"role"`
		Model       string `json:"model"`
		StopReason  string `json:"stop_reason"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Type != "message" {
		t.Errorf("type: %q", resp.Type)
	}
	if resp.Role != "assistant" {
		t.Errorf("role: %q", resp.Role)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello world" {
		t.Errorf("content: %+v", resp.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason: %q", resp.StopReason)
	}
}

func TestMessages_Streaming(t *testing.T) {
	h := New(okRouter("Hello", " world"))
	w := doPost(t, h, "/v1/messages", map[string]any{
		"model":      "claude",
		"max_tokens": 100,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type: %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "message_start") {
		t.Error("missing message_start event")
	}
	if !strings.Contains(body, "message_stop") {
		t.Error("missing message_stop event")
	}
	if !strings.Contains(body, "Hello") {
		t.Error("missing content in stream")
	}
}

func TestMessages_RouterError_503(t *testing.T) {
	h := New(errRouter("all down"))
	w := doPost(t, h, "/v1/messages", map[string]any{
		"model":      "claude",
		"max_tokens": 100,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", w.Code)
	}
	assertAnthropicError(t, w.Body.Bytes(), "api_error")
}

func TestMessages_SystemString(t *testing.T) {
	var gotMsgs []chat.ChatMessage
	captureRouter := &captureStub{chunks: []string{"ok"}, captured: &gotMsgs}
	h := New(captureRouter)
	doPost(t, h, "/v1/messages", map[string]any{
		"model":      "claude",
		"max_tokens": 100,
		"system":     "Be concise.",
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	})
	if len(gotMsgs) < 2 {
		t.Fatalf("expected ≥2 messages (system + user), got %d", len(gotMsgs))
	}
	if gotMsgs[0].Role != "system" || gotMsgs[0].Content != "Be concise." {
		t.Errorf("system message: %+v", gotMsgs[0])
	}
}

func TestMessages_ContentArray(t *testing.T) {
	var gotMsgs []chat.ChatMessage
	captureRouter := &captureStub{chunks: []string{"ok"}, captured: &gotMsgs}
	h := New(captureRouter)
	doPost(t, h, "/v1/messages", map[string]any{
		"model":      "claude",
		"max_tokens": 100,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "text", "text": "Hello"},
					{"type": "text", "text": "World"},
				},
			},
		},
	})
	if len(gotMsgs) == 0 {
		t.Fatal("no messages captured")
	}
	if gotMsgs[0].Content != "Hello\nWorld" {
		t.Errorf("content: %q", gotMsgs[0].Content)
	}
}

// ── POST /v1/messages/count_tokens ───────────────────────────────────────────

func TestCountTokens_Basic(t *testing.T) {
	h := New(okRouter())
	w := doPost(t, h, "/v1/messages/count_tokens", map[string]any{
		"messages": []map[string]string{
			{"role": "user", "content": "abcdefgh"}, // 8 chars = 2 tokens
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		InputTokens int `json:"input_tokens"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.InputTokens != 2 {
		t.Errorf("input_tokens: got %d, want 2", resp.InputTokens)
	}
}

func TestCountTokens_WithSystem(t *testing.T) {
	h := New(okRouter())
	w := doPost(t, h, "/v1/messages/count_tokens", map[string]any{
		"system":   "abcd",     // 4 chars = 1 token
		"messages": []map[string]string{{"role": "user", "content": "abcd"}}, // 4 chars = 1 token
	})
	var resp struct{ InputTokens int `json:"input_tokens"` }
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.InputTokens != 2 {
		t.Errorf("input_tokens: got %d, want 2", resp.InputTokens)
	}
}

func TestCountTokens_EmptyMessages(t *testing.T) {
	h := New(okRouter())
	w := doPost(t, h, "/v1/messages/count_tokens", map[string]any{
		"messages": []map[string]string{},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct{ InputTokens int `json:"input_tokens"` }
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.InputTokens != 0 {
		t.Errorf("input_tokens: got %d, want 0", resp.InputTokens)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertAnthropicError(t *testing.T, body []byte, wantErrType string) {
	t.Helper()
	var resp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	if resp.Type != "error" {
		t.Errorf("top-level type: %q", resp.Type)
	}
	if resp.Error.Type != wantErrType {
		t.Errorf("error.type: got %q, want %q", resp.Error.Type, wantErrType)
	}
	if resp.Error.Message == "" {
		t.Error("error.message is empty")
	}
}

// captureStub records the messages passed to Handle.
type captureStub struct {
	chunks   []string
	captured *[]chat.ChatMessage
}

func (c *captureStub) Handle(_ context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	*c.captured = append(*c.captured, msgs...)
	ch := make(chan chat.StreamChunk, len(c.chunks))
	for _, t := range c.chunks {
		ch <- chat.StreamChunk{Text: t}
	}
	close(ch)
	return ch, nil
}
