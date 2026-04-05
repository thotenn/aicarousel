package legacy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thotenn/aicarousel/internal/chat"
)

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

func doPost(t *testing.T, h *Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, r)
	return w
}

func TestChat_PlainTextStream(t *testing.T) {
	h := New(&stubRouter{chunks: []string{"Hello", " world"}})
	w := doPost(t, h, []map[string]string{{"role": "user", "content": "hi"}})

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if body := w.Body.String(); body != "Hello world" {
		t.Errorf("body: got %q, want %q", body, "Hello world")
	}
}

func TestChat_InvalidBody_400(t *testing.T) {
	h := New(&stubRouter{})
	r := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestChat_EmptyMessages_400(t *testing.T) {
	h := New(&stubRouter{chunks: []string{"hi"}})
	w := doPost(t, h, []map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestChat_RouterError_503(t *testing.T) {
	h := New(&stubRouter{err: errTest("all down")})
	w := doPost(t, h, []map[string]string{{"role": "user", "content": "hi"}})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", w.Code)
	}
}

func TestChat_NoSSEFraming(t *testing.T) {
	h := New(&stubRouter{chunks: []string{"chunk1", "chunk2"}})
	w := doPost(t, h, []map[string]string{{"role": "user", "content": "hi"}})
	body := w.Body.String()
	// No "data: " prefix — plain text only.
	if len(body) > 0 && body[:5] == "data:" {
		t.Errorf("legacy /chat must not use SSE framing, got: %s", body)
	}
	if body != "chunk1chunk2" {
		t.Errorf("body: got %q, want %q", body, "chunk1chunk2")
	}
}

func TestChat_StreamError_StopsGracefully(t *testing.T) {
	// Router returns a channel that emits an error chunk — handler must stop.
	h := New(&errChunkRouter{})
	w := doPost(t, h, []map[string]string{{"role": "user", "content": "hi"}})
	// The response may be 200 with partial body or empty; just ensure no panic.
	if w.Code == http.StatusBadRequest {
		t.Errorf("unexpected 400, got body: %s", w.Body.String())
	}
}

// errChunkRouter returns a channel that emits one error chunk then closes.
type errChunkRouter struct{}

func (e *errChunkRouter) Handle(_ context.Context, _ []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	ch := make(chan chat.StreamChunk, 1)
	ch <- chat.StreamChunk{Err: errors.New("provider died")}
	close(ch)
	return ch, nil
}
