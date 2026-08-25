package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/thotenn/aicarousel/internal/chat"
	anthfmt "github.com/thotenn/aicarousel/internal/formatters/anthropic"
	"github.com/thotenn/aicarousel/internal/httpapi"
	"github.com/thotenn/aicarousel/internal/util/id"
)

// chatRouter is the minimal interface handlers require from the chat.Router.
type chatRouter interface {
	Handle(ctx context.Context, msgs []chat.ChatMessage, opts chat.Options) (<-chan chat.StreamChunk, error)
}

// Handler holds dependencies for Anthropic-compatible HTTP endpoints.
type Handler struct {
	router chatRouter
}

// New creates a Handler backed by the given router.
func New(r chatRouter) *Handler { return &Handler{router: r} }

// Register mounts Anthropic routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/messages", h.messages)
	mux.HandleFunc("POST /v1/messages/count_tokens", h.countTokens)
}

// ── request body ─────────────────────────────────────────────────────────────

type messagesRequest struct {
	Model       string          `json:"model"`
	Messages    []anthMessage   `json:"messages"`
	System      json.RawMessage `json:"system"`     // string | [{type,text}]
	MaxTokens   *int            `json:"max_tokens"` // required
	Stream      *bool           `json:"stream"`
	Temperature *float64        `json:"temperature"`
	TopP        *float64        `json:"top_p"`
}

type anthMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string | [{type,text}]
}

type countTokensRequest struct {
	Messages []anthMessage   `json:"messages"`
	System   json.RawMessage `json:"system"`
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	var req messagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", "invalid_request_error", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, "messages: Input should be a valid list", "invalid_request_error", http.StatusBadRequest)
		return
	}

	if req.MaxTokens == nil {
		writeError(w, "max_tokens: Field required", "invalid_request_error", http.StatusBadRequest)
		return
	}

	// Build internal message list.
	var msgs []chat.ChatMessage

	// Prepend system if present.
	if len(req.System) > 0 && string(req.System) != "null" {
		sys := extractText(req.System)
		if sys != "" {
			msgs = append(msgs, chat.ChatMessage{Role: "system", Content: sys})
		}
	}

	for _, m := range req.Messages {
		msgs = append(msgs, chat.ChatMessage{
			Role:    m.Role,
			Content: extractText(m.Content),
		})
	}

	model := req.Model
	if model == "" {
		model = "aicarousel"
	}

	// stream defaults to false for Anthropic
	shouldStream := req.Stream != nil && *req.Stream

	opts := chat.Options{
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
	}

	ch, err := h.router.Handle(r.Context(), msgs, opts)
	if err != nil {
		writeError(w, "All AI services failed", "api_error", http.StatusServiceUnavailable)
		return
	}

	msgID := id.NewMsgID()

	if shouldStream {
		flusher, ok := httpapi.AssertFlusher(w)
		if !ok {
			return
		}
		httpapi.SetSSEHeaders(w)
		if err := anthfmt.Stream(w, flusher.Flush, msgID, model, ch); err != nil {
			return
		}
		return
	}

	// Non-streaming.
	resp, err := anthfmt.Complete(msgID, model, ch)
	if err != nil {
		writeError(w, "All AI services failed", "api_error", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

func (h *Handler) countTokens(w http.ResponseWriter, r *http.Request) {
	var req countTokensRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", "invalid_request_error", http.StatusBadRequest)
		return
	}

	var total int
	if len(req.System) > 0 && string(req.System) != "null" {
		total += len(extractText(req.System))
	}
	for _, m := range req.Messages {
		total += len(extractText(m.Content))
	}

	inputTokens := (total + 3) / 4

	w.Header().Set("Content-Type", "application/json")
	_ = time.Now()                     // ensure time import is used (used in messages handler via formatter)
	json.NewEncoder(w).Encode(struct { //nolint:errcheck
		InputTokens int `json:"input_tokens"`
	}{inputTokens})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractText extracts plain text from an Anthropic content field, which may
// be a JSON string or an array of {type,text} blocks.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first.
	var s string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// Try array of content blocks.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func writeError(w http.ResponseWriter, msg, errType string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(anthfmt.ErrorJSON(msg, errType)) //nolint:errcheck
}
