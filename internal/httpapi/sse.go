package httpapi

import "net/http"

// SetSSEHeaders sets the standard headers required for text/event-stream
// responses. Call before writing any data.
func SetSSEHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
}

// AssertFlusher type-asserts w to http.Flusher. If the assertion fails it
// writes a 500 and returns (nil, false). Callers must return immediately
// when ok is false.
func AssertFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported by underlying writer", http.StatusInternalServerError)
	}
	return f, ok
}
