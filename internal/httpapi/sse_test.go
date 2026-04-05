package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetSSEHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	SetSSEHeaders(w)

	h := w.Header()
	if ct := h.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := h.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if conn := h.Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
	if xab := h.Get("X-Accel-Buffering"); xab != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", xab)
	}
}

func TestAssertFlusher_Success(t *testing.T) {
	w := httptest.NewRecorder()
	f, ok := AssertFlusher(w)
	if !ok {
		t.Fatal("AssertFlusher returned false on httptest.ResponseRecorder (which implements http.Flusher)")
	}
	if f == nil {
		t.Fatal("returned Flusher is nil")
	}
}

// nonFlusher wraps ResponseWriter but deliberately does NOT implement Flusher.
type nonFlusher struct{ http.ResponseWriter }

func TestAssertFlusher_Failure(t *testing.T) {
	w := httptest.NewRecorder()
	nf := &nonFlusher{w}
	f, ok := AssertFlusher(nf)
	if ok {
		t.Fatal("expected ok=false for non-Flusher writer")
	}
	if f != nil {
		t.Fatal("expected nil Flusher on failure")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "streaming unsupported") {
		t.Errorf("body %q missing expected message", w.Body.String())
	}
}
