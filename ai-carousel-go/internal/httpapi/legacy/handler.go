package legacy

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thotenn/aicarousel-go/internal/chat"
)

// chatRouter is the minimal interface the handler requires.
type chatRouter interface {
	Handle(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error)
}

// Handler holds dependencies for the legacy /chat endpoint.
type Handler struct {
	router chatRouter
}

// New creates a Handler backed by the given router.
func New(r chatRouter) *Handler { return &Handler{router: r} }

// Register mounts the legacy route on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /chat", h.chat)
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var msgs []chat.ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil || len(msgs) == 0 {
		http.Error(w, "invalid request body: expected non-empty array of messages",
			http.StatusBadRequest)
		return
	}

	ch, err := h.router.Handle(r.Context(), msgs)
	if err != nil {
		http.Error(w, "All providers exhausted: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Plain-text streaming — no SSE framing.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	for chunk := range ch {
		if chunk.Err != nil {
			// Stream already started; nothing more we can do.
			return
		}
		if chunk.Text == "" {
			continue
		}
		if _, err := w.Write([]byte(chunk.Text)); err != nil {
			return
		}
		flusher.Flush()
	}
}
